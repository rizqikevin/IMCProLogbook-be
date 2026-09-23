import { useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { useAuth } from "../auth";
import { useBook, usePhoto } from "../hooks";
import { request } from "../lib/api";
import { formatBytes, formatDate, formatTime, limits } from "../lib/format";
import { Dialog, ErrorNotice, Icon, Loading } from "../components/common";

export default function Book() {
  const { id } = useParams();
  const { user } = useAuth();
  const { book, loading, error, retry } = useBook(id);
  const [index, setIndex] = useState(0);
  const [zoom, setZoom] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState(null);
  const navigate = useNavigate();
  const location = useLocation();
  const selected =
    book?.photos?.[Math.min(index, (book?.photos?.length || 1) - 1)];
  const photo = usePhoto(id, selected?.id);
  async function remove() {
    setDeleting(true);
    setDeleteError(null);
    try {
      await request(`/logbooks/${id}`, { method: "DELETE" });
      navigate("/", { replace: true });
    } catch (error) {
      setDeleteError(error);
    } finally {
      setDeleting(false);
    }
  }
  return (
    <div className="page book-page">
      <Link to="/" className="back-link">
        <Icon name="back" />
        Semua arsip
      </Link>
      {loading ? (
        <Loading />
      ) : error ? (
        <ErrorNotice error={error} onRetry={retry} />
      ) : (
        <>
          {location.state?.saved && (
            <div className="notice notice-success" role="status">
              <Icon name="check" />
              <span>
                Arsip berhasil disimpan. Seluruh foto sudah terunggah.
              </span>
            </div>
          )}
          <div className="page-heading">
            <div>
              <p className="section-label">
                {formatDate(book.log_date)} · {book.shift.name}
              </p>
              <h1>{book.machine.name}</h1>
              <p className="muted">{book.photo_count} halaman logbook</p>
            </div>
            {book.photo_count < limits.totalPhotos && (
              <Link
                to={`/logbooks/${id}/add`}
                className="button button-primary"
              >
                <Icon name="plus" />
                Tambah halaman
              </Link>
            )}
          </div>
          <div className="book-workspace">
            <section className="page-viewer" aria-label="Foto logbook">
              <div className="viewer-toolbar">
                <strong>
                  Halaman {selected?.page_number || 0}{" "}
                  <span className="muted">/ {book.photo_count}</span>
                </strong>
                <div className="viewer-actions">
                  <button
                    className="text-button"
                    onClick={() => setZoom(true)}
                    disabled={!photo.url}
                  >
                    Perbesar
                  </button>
                  {photo.url && (
                    <a
                      className="text-button"
                      href={photo.url}
                      download={`${book.machine.name}-${book.log_date}-halaman-${selected.page_number}.${selected.content_type.split("/")[1]}`}
                    >
                      Unduh foto
                    </a>
                  )}
                </div>
              </div>
              <div className="photo-stage">
                {photo.loading ? (
                  <Loading>Memuat halaman…</Loading>
                ) : photo.error ? (
                  <ErrorNotice error={photo.error} onRetry={photo.retry} />
                ) : (
                  <button
                    className="photo-open"
                    onClick={() => setZoom(true)}
                    aria-label={`Perbesar halaman ${selected.page_number}`}
                  >
                    <img
                      src={photo.url}
                      alt={`Logbook ${book.machine.name}, ${formatDate(book.log_date)}, halaman ${selected.page_number}`}
                    />
                  </button>
                )}
              </div>
              <div className="viewer-pagination">
                <button
                  className="button button-outline"
                  disabled={index === 0}
                  onClick={() => setIndex((value) => value - 1)}
                >
                  <Icon name="back" />
                  <span>Sebelumnya</span>
                </button>
                <span>
                  {selected?.page_number} dari {book.photo_count}
                </span>
                <button
                  className="button button-outline"
                  disabled={index >= book.photo_count - 1}
                  onClick={() => setIndex((value) => value + 1)}
                >
                  <span>Berikutnya</span>
                  <Icon name="next" />
                </button>
              </div>
            </section>
            <aside className="book-info">
              <section>
                <h2>Informasi arsip</h2>
                <dl>
                  <div>
                    <dt>Mesin</dt>
                    <dd>{book.machine.name}</dd>
                  </div>
                  <div>
                    <dt>Tanggal logbook</dt>
                    <dd>{formatDate(book.log_date)}</dd>
                  </div>
                  <div>
                    <dt>Shift</dt>
                    <dd>{book.shift.name}</dd>
                  </div>
                  <div>
                    <dt>Diarsipkan</dt>
                    <dd>{formatTime(book.created_at)}</dd>
                  </div>
                  <div>
                    <dt>Diperbarui</dt>
                    <dd>{formatTime(book.updated_at)}</dd>
                  </div>
                  {book.created_by === user.id && (
                    <div>
                      <dt>Dibuat oleh</dt>
                      <dd>{user.name}</dd>
                    </div>
                  )}
                </dl>
              </section>
              <section className="page-index">
                <h2>Pilih halaman</h2>
                <div className="page-index-buttons">
                  {book.photos.map((item, i) => (
                    <button
                      key={item.id}
                      aria-label={`Lihat halaman ${item.page_number}`}
                      aria-pressed={i === index}
                      className={i === index ? "selected" : ""}
                      onClick={() => setIndex(i)}
                    >
                      {String(item.page_number).padStart(2, "0")}
                    </button>
                  ))}
                </div>
                {selected && (
                  <p className="muted">
                    {selected.content_type.split("/")[1].toUpperCase()} ·{" "}
                    {formatBytes(selected.size)}
                  </p>
                )}
              </section>
              {user.role === "admin" && (
                <section className="delete-section">
                  <button
                    className="text-button danger-text"
                    onClick={() => setConfirm(true)}
                  >
                    <Icon name="trash" />
                    Hapus arsip
                  </button>
                  <p>Seluruh halaman dalam arsip ini akan dihapus.</p>
                </section>
              )}
            </aside>
          </div>
          {zoom && (
            <Dialog
              title={`Halaman ${selected.page_number} · ${book.machine.name}`}
              className="photo-dialog"
              onClose={() => setZoom(false)}
            >
              <img
                src={photo.url}
                alt={`Logbook halaman ${selected.page_number}`}
              />
            </Dialog>
          )}
          {confirm && (
            <Dialog
              title="Hapus arsip ini?"
              onClose={() => setConfirm(false)}
              dismissible={!deleting}
            >
              <p>
                Arsip <strong>{book.machine.name}</strong>,{" "}
                {formatDate(book.log_date)}, {book.shift.name}, beserta{" "}
                {book.photo_count} halamannya akan dihapus. Tindakan ini tidak
                dapat dibatalkan.
              </p>
              <ErrorNotice error={deleteError} />
              <div className="dialog-actions">
                <button
                  className="button button-outline"
                  disabled={deleting}
                  onClick={() => setConfirm(false)}
                >
                  Batal
                </button>
                <button
                  className="button button-danger"
                  disabled={deleting}
                  onClick={remove}
                >
                  {deleting ? "Menghapus…" : "Hapus arsip"}
                </button>
              </div>
            </Dialog>
          )}
        </>
      )}
    </div>
  );
}
