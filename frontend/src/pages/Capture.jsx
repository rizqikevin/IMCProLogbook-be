import { useEffect, useRef, useState } from "react";
import { Link, useBlocker, useNavigate, useParams } from "react-router-dom";
import { useBook, useCatalog } from "../hooks";
import { queryString, request, uploadLogbook } from "../lib/api";
import {
  formatBytes,
  formatDate,
  limits,
  today,
  validateSelection,
} from "../lib/format";
import { Dialog, ErrorNotice, Icon, Loading } from "../components/common";
import Camera from "../components/Camera";

export default function Capture() {
  const { id } = useParams();
  const navigate = useNavigate();
  const catalog = useCatalog();
  const detail = useBook(id);
  const [machine, setMachine] = useState("");
  const [shift, setShift] = useState("");
  const [date, setDate] = useState(today);
  const [files, setFiles] = useState([]);
  const filesRef = useRef([]);
  const [camera, setCamera] = useState(false);
  const [preparing, setPreparing] = useState(false);
  const preparingRef = useRef(false);
  const [uploading, setUploading] = useState(false);
  const uploadingRef = useRef(false);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState(null);
  const [uncertain, setUncertain] = useState(false);
  const [checking, setChecking] = useState(false);
  const [existingId, setExistingId] = useState(null);
  const [preview, setPreview] = useState(null);
  const gallery = useRef(null);
  const nativeCamera = useRef(null);
  const uploadController = useRef(null);
  const saved = useRef(false);
  const mounted = useRef(true);
  const baseline = useRef(0);
  const blocker = useBlocker(
    () => filesRef.current.length > 0 && !saved.current,
  );
  const maximum = Math.min(
    limits.photos,
    limits.totalPhotos - (detail.book?.photo_count || 0),
  );
  const busy = uploading || preparing || checking;
  const totalBytes = files.reduce((total, item) => total + item.file.size, 0);

  useEffect(() => {
    function beforeUnload(event) {
      if (filesRef.current.length && !saved.current) {
        event.preventDefault();
        event.returnValue = "";
      }
    }
    window.addEventListener("beforeunload", beforeUnload);
    return () => window.removeEventListener("beforeunload", beforeUnload);
  }, []);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      uploadController.current?.abort();
      filesRef.current.forEach((item) => URL.revokeObjectURL(item.url));
    };
  }, []);

  function updateFiles(next) {
    filesRef.current = next;
    setFiles(next);
  }
  async function addFiles(incoming) {
    if (preparingRef.current || uploadingRef.current || uncertain) return;
    const selection = Array.from(incoming);
    if (!selection.length) return;
    const invalid = validateSelection(filesRef.current, selection, maximum);
    if (invalid) {
      setError(invalid);
      return;
    }
    preparingRef.current = true;
    setPreparing(true);
    setError(null);
    const added = [];
    try {
      for (const file of selection) {
        const bitmap = await createImageBitmap(file).catch(() => {
          throw new Error(
            `Foto “${file.name}” tidak dapat dibaca. Gunakan JPG, PNG, atau WebP yang valid.`,
          );
        });
        const pixels = bitmap.width * bitmap.height;
        bitmap.close();
        if (pixels > limits.pixels)
          throw new Error(
            `Foto “${file.name}” melebihi 40 megapiksel. Perkecil foto terlebih dahulu.`,
          );
        let sha256 = "";
        if (crypto.subtle) {
          const digest = await crypto.subtle.digest(
            "SHA-256",
            await file.arrayBuffer(),
          );
          sha256 = [...new Uint8Array(digest)]
            .map((byte) => byte.toString(16).padStart(2, "0"))
            .join("");
        }
        added.push({
          id: `${Date.now()}-${Math.random()}`,
          file,
          sha256,
          url: URL.createObjectURL(file),
        });
      }
      if (mounted.current) updateFiles([...filesRef.current, ...added]);
      else added.forEach((item) => URL.revokeObjectURL(item.url));
    } catch (error) {
      added.forEach((item) => URL.revokeObjectURL(item.url));
      setError(error);
    } finally {
      preparingRef.current = false;
      setPreparing(false);
    }
  }
  function remove(index) {
    URL.revokeObjectURL(files[index].url);
    updateFiles(files.filter((_, current) => current !== index));
  }
  function move(index, direction) {
    const next = [...files];
    [next[index], next[index + direction]] = [
      next[index + direction],
      next[index],
    ];
    updateFiles(next);
  }
  function complete(book) {
    saved.current = true;
    navigate(`/logbooks/${book.id}`, { replace: true, state: { saved: true } });
  }
  async function submit(event) {
    event.preventDefault();
    if (uploadingRef.current || busy || uncertain) return;
    if (!files.length) {
      setError("Tambahkan setidaknya satu foto logbook.");
      return;
    }
    setError(null);
    setExistingId(null);
    setProgress(0);
    setUploading(true);
    uploadingRef.current = true;
    baseline.current = detail.book?.photo_count || 0;
    const controller = new AbortController();
    uploadController.current = controller;
    try {
      complete(
        await uploadLogbook({
          bookId: id,
          machineId: machine,
          date,
          shiftId: shift,
          files: files.map((item) => item.file),
          signal: controller.signal,
          onProgress: setProgress,
        }),
      );
    } catch (error) {
      if (error.name === "AbortError") return;
      setError(error);
      if (
        !error.status ||
        error.status >= 500 ||
        (error.status >= 200 && error.status < 300)
      )
        setUncertain(true);
      if (error.code === "conflict") setExistingId("find");
    } finally {
      setUploading(false);
      uploadingRef.current = false;
    }
  }
  async function checkResult() {
    setChecking(true);
    setError(null);
    try {
      let book;
      if (id) book = await request(`/logbooks/${id}`);
      else {
        const page = await request(
          `/logbooks?${queryString({ machine_id: machine, shift_id: shift, date_from: date, date_to: date, limit: 1 })}`,
        );
        if (page.items[0])
          book = await request(`/logbooks/${page.items[0].id}`);
      }
      if (book) {
        const candidates = book.photos.slice(id ? baseline.current : 0);
        const hashes = files.map((item) => item.sha256);
        const matched =
          hashes.every(Boolean) &&
          candidates.some((_, start) =>
            hashes.every(
              (hash, offset) => candidates[start + offset]?.sha256 === hash,
            ),
          );
        if (matched) {
          complete(book);
          return;
        }
        setExistingId(book.id);
        setError(
          "Arsip ditemukan, tetapi foto yang dipilih belum bisa dipastikan sudah tersimpan. Periksa isi arsip sebelum mengunggah kembali.",
        );
      } else {
        if (!id) setUncertain(false);
        setError(
          "Arsip belum ditemukan. Foto pilihan Anda masih tersedia di halaman ini.",
        );
      }
    } catch (error) {
      setError(error);
    } finally {
      setChecking(false);
    }
  }
  const selectedMachine = id
    ? detail.book?.machine.name
    : catalog.machines.find((item) => String(item.id) === machine)?.name;
  return (
    <div className="page capture-page">
      <Link to={id ? `/logbooks/${id}` : "/"} className="back-link">
        <Icon name="back" />
        {id ? "Kembali ke arsip" : "Semua arsip"}
      </Link>
      <div className="page-heading">
        <div>
          <p className="section-label">
            {id ? "Lanjutkan catatan" : "Dokumentasi produksi"}
          </p>
          <h1>{id ? "Tambah halaman" : "Arsipkan logbook"}</h1>
          <p className="muted">
            Ambil foto satu per satu. Simpan semuanya saat sudah selesai.
          </p>
        </div>
        <span className="draft-label">Belum disimpan</span>
      </div>
      {catalog.loading || (id && detail.loading) ? (
        <Loading>Memuat data mesin dan arsip…</Loading>
      ) : catalog.error || detail.error ? (
        <ErrorNotice
          error={catalog.error || detail.error}
          onRetry={() => {
            catalog.retry();
            detail.retry();
          }}
        />
      ) : (
        <form onSubmit={submit} className="capture-form">
          <section className="entry-metadata">
            <div className="section-heading">
              <span className="step-number">01</span>
              <div>
                <h2>Identitas logbook</h2>
                <p>Tentukan catatan untuk mesin dan shift yang sesuai.</p>
              </div>
            </div>
            {id ? (
              <div className="existing-metadata">
                <strong>{detail.book.machine.name}</strong>
                <span>{formatDate(detail.book.log_date)}</span>
                <span>{detail.book.shift.name}</span>
                <span>{detail.book.photo_count} halaman tersimpan</span>
              </div>
            ) : (
              <div className="metadata-fields">
                <label>
                  Mesin
                  <select
                    value={machine}
                    onChange={(event) => setMachine(event.target.value)}
                    required
                    disabled={busy || uncertain}
                  >
                    <option value="" disabled>
                      Pilih mesin
                    </option>
                    {catalog.machines.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  Tanggal logbook
                  <input
                    type="date"
                    value={date}
                    onChange={(event) => setDate(event.target.value)}
                    required
                    disabled={busy || uncertain}
                  />
                </label>
                <label>
                  Shift
                  <select
                    value={shift}
                    onChange={(event) => setShift(event.target.value)}
                    required
                    disabled={busy || uncertain}
                  >
                    <option value="" disabled>
                      Pilih shift
                    </option>
                    {catalog.shifts.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
            )}
          </section>
          <section className="entry-photos">
            <div className="section-heading">
              <span className="step-number">02</span>
              <div>
                <h2>Halaman logbook</h2>
                <p>Urutan foto menjadi urutan halaman dalam arsip.</p>
              </div>
              <span className="photo-tally">
                {files.length} / {maximum}
              </span>
            </div>
            <ErrorNotice error={error} />
            {(uncertain || existingId) && (
              <div className="notice notice-warning">
                <div>
                  <strong>Periksa arsip sebelum mengunggah ulang</strong>
                  <p>
                    Jangan mengirim ulang halaman yang sama sebelum hasil
                    unggahan dipastikan.
                  </p>
                  <div className="inline-actions">
                    <button
                      type="button"
                      className="button button-outline"
                      onClick={checkResult}
                      disabled={busy}
                    >
                      {checking ? "Memeriksa…" : "Periksa hasil unggahan"}
                    </button>
                    {existingId && existingId !== "find" && (
                      <Link
                        className="text-button"
                        to={`/logbooks/${existingId}`}
                      >
                        Buka arsip yang ditemukan
                      </Link>
                    )}
                  </div>
                </div>
              </div>
            )}
            <input
              ref={gallery}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              multiple
              className="visually-hidden"
              tabIndex={-1}
              aria-label="Pilih foto logbook"
              onChange={(event) => {
                addFiles(event.target.files);
                event.target.value = "";
              }}
            />
            <input
              ref={nativeCamera}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              capture="environment"
              className="visually-hidden"
              tabIndex={-1}
              aria-label="Kamera perangkat"
              onChange={(event) => {
                addFiles(event.target.files);
                event.target.value = "";
              }}
            />
            <div
              className={`photo-input-area ${files.length ? "has-photos" : ""}`}
            >
              <div className="capture-invitation">
                <Icon name="camera" size={32} />
                <div>
                  <h3>
                    {files.length
                      ? "Masih ada halaman lain?"
                      : "Mulai dari halaman pertama."}
                  </h3>
                  <p>
                    {files.length
                      ? "Tambahkan foto berikutnya sebelum menyimpan."
                      : "Buka kamera atau pilih foto yang sudah diambil."}
                  </p>
                </div>
              </div>
              <div className="photo-input-actions">
                <button
                  type="button"
                  className="button button-primary"
                  onClick={() => setCamera(true)}
                  disabled={busy || uncertain || files.length >= maximum}
                >
                  <Icon name="camera" />
                  Buka kamera
                </button>
                <button
                  type="button"
                  className="button button-outline"
                  onClick={() => gallery.current.click()}
                  disabled={busy || uncertain || files.length >= maximum}
                >
                  <Icon name="upload" />
                  Pilih foto
                </button>
              </div>
              <button
                type="button"
                className="text-button native-camera-button"
                onClick={() => nativeCamera.current.click()}
                disabled={busy || uncertain || files.length >= maximum}
              >
                Gunakan kamera perangkat
              </button>
            </div>
            {preparing && <Loading>Memeriksa foto pilihan…</Loading>}
            {files.length > 0 && (
              <ol className="draft-photos">
                {files.map((item, index) => (
                  <li key={item.id} className="draft-photo">
                    <button
                      className="draft-photo-preview"
                      type="button"
                      onClick={() => setPreview(item)}
                      aria-label={`Pratinjau halaman ${(detail.book?.photo_count || 0) + index + 1}`}
                    >
                      <img
                        src={item.url}
                        alt={`Foto pilihan halaman ${index + 1}`}
                      />
                    </button>
                    <div className="draft-photo-caption">
                      <strong>
                        Halaman {(detail.book?.photo_count || 0) + index + 1}
                      </strong>
                      <span>{formatBytes(item.file.size)}</span>
                    </div>
                    <div className="draft-photo-actions">
                      <button
                        type="button"
                        className="icon-button"
                        aria-label={`Pindahkan foto ${index + 1} lebih awal`}
                        onClick={() => move(index, -1)}
                        disabled={busy || uncertain || index === 0}
                      >
                        <Icon name="up" />
                      </button>
                      <button
                        type="button"
                        className="icon-button"
                        aria-label={`Pindahkan foto ${index + 1} lebih akhir`}
                        onClick={() => move(index, 1)}
                        disabled={
                          busy || uncertain || index === files.length - 1
                        }
                      >
                        <Icon name="down" />
                      </button>
                      <button
                        type="button"
                        className="icon-button danger-text"
                        aria-label={`Hapus foto ${index + 1}`}
                        onClick={() => remove(index)}
                        disabled={busy || uncertain}
                      >
                        <Icon name="trash" />
                      </button>
                    </div>
                  </li>
                ))}
              </ol>
            )}
            <p className="upload-guidance">
              JPG, PNG, atau WebP · Maks. 10 MB per foto · Maks. {maximum} foto
              per unggahan
            </p>
          </section>
          <div className="save-bar">
            <div>
              <strong>
                {files.length
                  ? `${files.length} halaman siap disimpan`
                  : "Belum ada foto dipilih"}
              </strong>
              <span>
                {selectedMachine || "Pilih mesin terlebih dahulu"}
                {files.length > 0 && ` · ${formatBytes(totalBytes)}`}
              </span>
            </div>
            <button
              className="button button-primary save-button"
              type="submit"
              disabled={busy || uncertain || !files.length}
            >
              <Icon name="check" />
              {uploading ? "Menyimpan arsip…" : "Done · Simpan arsip"}
            </button>
            {uploading && (
              <div className="upload-progress" role="status">
                <progress
                  max="100"
                  value={progress}
                  aria-label="Progres unggahan"
                />
                <span>
                  {progress < 100
                    ? `Mengunggah foto ${progress}%`
                    : "Foto terkirim. Menunggu konfirmasi penyimpanan…"}
                </span>
              </div>
            )}
          </div>
          <p className="draft-note">
            Foto belum diunggah sebelum Anda menekan Done. Pilihan foto tidak
            disimpan jika halaman ditutup.
          </p>
        </form>
      )}
      {camera && (
        <Camera
          onClose={() => setCamera(false)}
          onCapture={addFiles}
          count={files.length}
          maximum={maximum}
        />
      )}
      {preview && (
        <Dialog
          title="Pratinjau foto pilihan"
          className="photo-dialog"
          onClose={() => setPreview(null)}
        >
          <img src={preview.url} alt="Foto logbook yang akan diunggah" />
        </Dialog>
      )}
      {blocker.state === "blocked" && (
        <Dialog
          title={
            uploading ? "Unggahan sedang berjalan" : "Tinggalkan foto pilihan?"
          }
          onClose={() => blocker.reset()}
          dismissible={!uploading}
        >
          <p>
            {uploading
              ? "Tunggu sampai server mengonfirmasi bahwa arsip tersimpan."
              : "Foto yang belum disimpan akan hilang dari halaman ini."}
          </p>
          <div className="dialog-actions">
            <button
              className="button button-primary"
              onClick={() => blocker.reset()}
            >
              {uploading ? "Tunggu unggahan" : "Lanjutkan mengarsipkan"}
            </button>
            {!uploading && (
              <button
                className="button button-outline"
                onClick={() => blocker.proceed()}
              >
                Tinggalkan halaman
              </button>
            )}
          </div>
        </Dialog>
      )}
    </div>
  );
}
