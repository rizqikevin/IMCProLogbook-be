import { useEffect } from "react";
import { Dialog, ErrorNotice, Icon, Loading } from "./common";

export default function PhotoDialog({
  title,
  subtitle,
  caption,
  photoUrl,
  loading = false,
  error = null,
  onRetry,
  downloadName,
  onPrev,
  onNext,
  hasPrev = false,
  hasNext = false,
  counterText,
  onClose,
}) {
  useEffect(() => {
    function handleKeyDown(event) {
      if (event.key === "ArrowLeft" && hasPrev && onPrev) {
        event.preventDefault();
        onPrev();
      } else if (event.key === "ArrowRight" && hasNext && onNext) {
        event.preventDefault();
        onNext();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [hasPrev, hasNext, onPrev, onNext]);

  return (
    <Dialog title={title} className="photo-dialog" onClose={onClose}>
      {subtitle && <p className="photo-dialog-subtitle">{subtitle}</p>}
      <div className="photo-dialog-stage">
        {loading ? (
          <Loading>Memuat foto…</Loading>
        ) : error ? (
          <ErrorNotice error={error} onRetry={onRetry} />
        ) : photoUrl ? (
          <img src={photoUrl} alt={title} />
        ) : null}
      </div>
      {caption && <p className="photo-dialog-caption">{caption}</p>}
      <div className="photo-dialog-nav">
        <div className="photo-dialog-pager">
          <button
            type="button"
            className="button button-outline"
            disabled={!hasPrev || !onPrev}
            onClick={onPrev}
            aria-label="Foto sebelumnya"
          >
            <Icon name="back" />
            <span>Sebelumnya</span>
          </button>
          {counterText && (
            <span className="photo-dialog-counter">{counterText}</span>
          )}
          <button
            type="button"
            className="button button-outline"
            disabled={!hasNext || !onNext}
            onClick={onNext}
            aria-label="Foto berikutnya"
          >
            <span>Berikutnya</span>
            <Icon name="next" />
          </button>
        </div>
        {photoUrl && downloadName && (
          <a
            className="button button-outline"
            href={photoUrl}
            download={downloadName}
          >
            Unduh foto
          </a>
        )}
      </div>
    </Dialog>
  );
}
