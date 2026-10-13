import { useEffect, useRef, useState } from "react";
import { Dialog, Icon, Loading } from "./common";

export default function Camera({ onClose, onCapture, onRemove, photos, maximum }) {
  const count = photos.length;
  const video = useRef(null);
  const thumbnails = useRef(null);
  const picker = useRef(null);
  const mounted = useRef(false);
  const capturing = useRef(false);
  const shutter = useRef(null);
  const [selected, setSelected] = useState(null);
  const [captureError, setCaptureError] = useState(null);
  const reviewed = photos.find((photo) => photo.url === selected);
  const reviewedIndex = photos.indexOf(reviewed);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [facing, setFacing] = useState("environment");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (thumbnails.current) {
      thumbnails.current.scrollLeft = thumbnails.current.scrollWidth;
    }
  }, [count]);
  useEffect(() => {
    let active = true;
    let stream;
    let timeout;
    let playbackStarted = false;
    const preview = video.current;

    function release() {
      clearTimeout(timeout);
      preview.removeEventListener("loadeddata", updateReady);
      preview.removeEventListener("canplay", updateReady);
      preview.removeEventListener("playing", updateReady);
      preview.removeEventListener("error", playbackFailed);
      if (stream && preview.srcObject === stream) {
        preview.pause();
        preview.srcObject = null;
      }
      stream?.getTracks().forEach((track) => track.stop());
    }

    function fail(err) {
      if (!active) return;
      active = false;
      release();
      setReady(false);
      setError(
        err.name === "NotAllowedError"
          ? "Izin kamera ditolak. Izinkan kamera di pengaturan peramban atau pilih foto dari galeri."
          : err.name === "NotFoundError"
            ? "Kamera tidak ditemukan. Hubungkan kamera atau pilih foto dari galeri."
            : err.name === "NotReadableError"
              ? "Kamera sedang dipakai. Tutup aplikasi kamera lain lalu coba lagi."
              : err.message || "Gagal membuka kamera. Coba lagi atau pilih foto dari galeri.",
      );
    }

    function updateReady() {
      if (
        active && playbackStarted &&
        preview.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA &&
        preview.videoWidth > 0 && preview.videoHeight > 0
      ) {
        clearTimeout(timeout);
        setReady(true);
      }
    }

    function playbackFailed() {
      fail(new Error("Pratinjau kamera gagal diputar. Silakan coba lagi."));
    }

    setReady(false);
    setError(null);
    async function start() {
      if (!navigator.mediaDevices?.getUserMedia) {
        fail(new Error("Perangkat tidak memiliki kamera atau akses ditolak."));
        return;
      }
      timeout = setTimeout(
        () => fail(new Error("Akses kamera tidak merespons.")),
        10000,
      );
      try {
        const acquired = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: facing,
            width: { ideal: 3840 },
            height: { ideal: 2160 },
          },
          audio: false,
        });
        // Permission may resolve after timeout, camera switch, or dialog close.
        if (!active) {
          acquired.getTracks().forEach((track) => track.stop());
          return;
        }
        stream = acquired;
        preview.addEventListener("loadeddata", updateReady);
        preview.addEventListener("canplay", updateReady);
        preview.addEventListener("playing", updateReady);
        preview.addEventListener("error", playbackFailed);
        preview.srcObject = stream;
        await preview.play();
        playbackStarted = true;
        updateReady();
      } catch (err) {
        fail(err);
      }
    }
    start();
    return () => {
      active = false;
      release();
    };
  }, [facing, attempt]);

  async function capture() {
    if (!ready || capturing.current || count >= maximum) return;
    capturing.current = true;
    setBusy(true);
    setCaptureError(null);
    try {
      const canvas = document.createElement("canvas");
      canvas.width = video.current.videoWidth;
      canvas.height = video.current.videoHeight;
      if (!canvas.width || !canvas.height)
        throw new Error("Tunggu sampai tampilan kamera siap.");
      canvas
        .getContext("2d")
        .drawImage(video.current, 0, 0, canvas.width, canvas.height);
      const blob = await new Promise((resolve) =>
        canvas.toBlob(resolve, "image/jpeg", 0.94),
      );
      if (!mounted.current) return;
      if (!blob)
        throw new Error("Foto belum berhasil diambil. Silakan coba lagi.");
      await onCapture([
        new File([blob], `logbook-${Date.now()}.jpg`, { type: "image/jpeg" }),
      ]);
    } catch (error) {
      if (mounted.current) setCaptureError(error.message);
    } finally {
      capturing.current = false;
      if (mounted.current) setBusy(false);
    }
  }
  function pickPhotos(event) {
    const incoming = Array.from(event.target.files || []);
    event.target.value = "";
    if (!incoming.length) return;
    try {
      onCapture(incoming);
      setCaptureError(null);
    } catch (error) {
      setCaptureError(error.message);
    }
  }

  function returnToCamera() {
    setSelected(null);
    requestAnimationFrame(() => shutter.current?.focus());
  }

  return (
    <Dialog title="Kamera" className="camera-dialog" onClose={onClose}>
      <div className="camera-toolbar">
        <input id="camera-photos" ref={picker} type="file" accept="image/jpeg,image/png,image/webp" multiple hidden onChange={pickPhotos} />
        <button type="button" className="button button-outline" onClick={() => picker.current.click()} disabled={busy || count >= maximum}>Pilih dari galeri</button>
        <button type="button" className="button button-outline" disabled={busy} onClick={() => {
          setReady(false);
          setSelected(null);
          setFacing((value) => value === "environment" ? "user" : "environment");
        }}>Ganti kamera</button>
      </div>
      <div className="camera-preview">
        <video ref={video} muted playsInline aria-label="Pratinjau kamera" className={reviewed ? "camera-video-hidden" : ""} />
        {reviewed ? <img className="camera-review" src={reviewed.url} alt={`Foto pilihan ${reviewedIndex + 1}`} /> : error ? (
          <div className="camera-feedback" role="alert">
            <p>{error}</p>
            <button type="button" className="button button-outline" onClick={() => setAttempt((value) => value + 1)}>Coba lagi</button>
          </div>
        ) : !ready && <Loading>Membuka kamera…</Loading>}
      </div>
      <div className="camera-selection">
        <div className="camera-status" role="status">
          <span>{count}/{maximum} foto dipilih</span>
          <span>{count >= maximum ? "Batas foto tercapai" : "Belum dikirim"}</span>
        </div>
        {captureError && <p className="camera-capture-error" role="alert">{captureError}</p>}
        {count > 0 && <ol className="camera-thumbnails" aria-label="Foto yang sudah dipilih" ref={thumbnails}>
          {photos.map((photo, index) => <li key={photo.url}>
            <button type="button" disabled={busy} aria-label={`Lihat foto ${index + 1}`} aria-pressed={selected === photo.url} onClick={() => setSelected(photo.url)}>
              <img src={photo.url} alt="" />
              <span>{index + 1}</span>
            </button>
          </li>)}
        </ol>}
      </div>
      <div className={`camera-controls${reviewed ? " camera-controls-review" : ""}`}>
        {reviewed ? <>
          <button type="button" className="button button-outline" onClick={() => { onRemove(reviewed); returnToCamera(); }} aria-label={`Hapus foto ${reviewedIndex + 1}`}>Hapus foto</button>
          <button type="button" className="button button-outline" onClick={returnToCamera}>Kembali ke kamera</button>
        </> : <button ref={shutter} type="button" className="camera-shutter" onClick={capture} disabled={!ready || busy || count >= maximum} aria-label="Ambil foto" aria-busy={busy}>
          <Icon name="camera" size={30} />
          <span>{busy ? "Menyimpan…" : "Ambil foto"}</span>
        </button>}
        <button type="button" className="button button-primary camera-done" onClick={onClose} disabled={busy}>Selesai{count > 0 ? ` (${count} foto)` : ""}</button>
      </div>
    </Dialog>
  );
}
