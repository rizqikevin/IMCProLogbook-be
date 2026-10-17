import { useEffect, useRef, useState } from "react";
import { Dialog, ErrorNotice, Icon, Loading } from "./common";

const MAX_ZOOM = 4;

function PhotoViewer({ photoUrl, title }) {
  const stageRef = useRef(null);
  const imageRef = useRef(null);
  const pointers = useRef(new Map());
  const viewRef = useRef({ scale: 1, x: 0, y: 0 });
  const [view, setView] = useState(viewRef.current);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [dragging, setDragging] = useState(false);

  function updateView(next) {
    const stage = stageRef.current;
    const image = imageRef.current;
    const scale = Math.min(MAX_ZOOM, Math.max(1, next.scale));
    const maxX = Math.max(0, (image.offsetWidth * scale - stage.clientWidth) / 2);
    const maxY = Math.max(0, (image.offsetHeight * scale - stage.clientHeight) / 2);
    viewRef.current = {
      scale,
      x: Math.max(-maxX, Math.min(maxX, next.x)),
      y: Math.max(-maxY, Math.min(maxY, next.y)),
    };
    setView(viewRef.current);
  }

  function zoom(scale) {
    updateView({ ...viewRef.current, scale });
  }

  useEffect(() => {
    const observer = new ResizeObserver(() => updateView(viewRef.current));
    observer.observe(stageRef.current);
    return () => observer.disconnect();
  }, []);

  function pointerDown(event) {
    if (!ready || (event.pointerType === "mouse" && event.button !== 0)) return;
    event.preventDefault();
    event.currentTarget.focus({ preventScroll: true });
    event.currentTarget.setPointerCapture(event.pointerId);
    pointers.current.set(event.pointerId, { x: event.clientX, y: event.clientY });
    setDragging(true);
  }

  function pointerMove(event) {
    const points = pointers.current;
    const previous = points.get(event.pointerId);
    if (!previous) return;
    const current = { x: event.clientX, y: event.clientY };
    const other = [...points.entries()].find(([id]) => id !== event.pointerId)?.[1];
    const old = viewRef.current;
    if (other) {
      const before = Math.hypot(previous.x - other.x, previous.y - other.y);
      const after = Math.hypot(current.x - other.x, current.y - other.y);
      const scale = Math.min(MAX_ZOOM, Math.max(1, old.scale * (before ? after / before : 1)));
      const rect = event.currentTarget.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const ratio = scale / old.scale;
      updateView({
        scale,
        x: (current.x + other.x) / 2 - centerX - ((previous.x + other.x) / 2 - centerX - old.x) * ratio,
        y: (current.y + other.y) / 2 - centerY - ((previous.y + other.y) / 2 - centerY - old.y) * ratio,
      });
    } else {
      updateView({ ...old, x: old.x + current.x - previous.x, y: old.y + current.y - previous.y });
    }
    points.set(event.pointerId, current);
  }

  function pointerEnd(event) {
    pointers.current.delete(event.pointerId);
    setDragging(pointers.current.size > 0);
  }

  function handleKeyDown(event) {
    if (!ready) return;
    const steps = { ArrowLeft: [40, 0], ArrowRight: [-40, 0], ArrowUp: [0, 40], ArrowDown: [0, -40] };
    if (event.shiftKey && steps[event.key]) {
      event.preventDefault();
      event.stopPropagation();
      const [x, y] = steps[event.key];
      updateView({ ...viewRef.current, x: viewRef.current.x + x, y: viewRef.current.y + y });
    }
  }

  return (
    <>
      <div
        ref={stageRef}
        className={`photo-dialog-stage photo-dialog-viewport${view.scale > 1 ? " is-zoomed" : ""}${dragging ? " is-dragging" : ""}`}
        tabIndex={0}
        role="region"
        aria-label="Foto. Perbesar lalu geser, atau gunakan Shift dan tombol panah."
        onKeyDown={handleKeyDown}
        onPointerDown={pointerDown}
        onPointerMove={pointerMove}
        onPointerUp={pointerEnd}
        onPointerCancel={pointerEnd}
        onLostPointerCapture={pointerEnd}
      >
        <img
          ref={imageRef}
          src={photoUrl}
          alt={title}
          draggable={false}
          style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`, visibility: ready ? "visible" : "hidden" }}
          onLoad={() => { setReady(true); updateView(viewRef.current); }}
          onError={() => { setFailed(true); setReady(false); }}
        />
        {!ready && <div className="photo-dialog-image-status">{failed ? "Foto tidak dapat ditampilkan." : <Loading>Memuat foto…</Loading>}</div>}
      </div>
      <div className="photo-dialog-zoom" role="group" aria-label="Ukuran foto">
        <button type="button" className="photo-viewer-control" aria-label="Perkecil foto" title="Perkecil foto" disabled={!ready || view.scale <= 1} onClick={() => zoom(view.scale - 0.5)}><Icon name="minus" /></button>
        <output aria-live="polite" aria-label="Pembesaran foto">{Math.round(view.scale * 100)}%</output>
        <button type="button" className="photo-viewer-control" aria-label="Perbesar foto" title="Perbesar foto" disabled={!ready || view.scale >= MAX_ZOOM} onClick={() => zoom(view.scale + 0.5)}><Icon name="plus" /></button>
        <span className="photo-dialog-tool-divider" aria-hidden="true" />
        <button type="button" className="photo-viewer-control" aria-label="Atur ulang ukuran foto" title="Atur ulang ukuran foto (100%)" disabled={!ready || view.scale === 1} onClick={() => updateView({ scale: 1, x: 0, y: 0 })}><Icon name="fit" /></button>
      </div>
      <p className="photo-dialog-hint">Cubit untuk zoom · Geser saat diperbesar<span> · Shift + panah untuk menggeser</span></p>
    </>
  );
}

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
      {loading ? (
        <div className="photo-dialog-stage"><Loading>Memuat foto…</Loading></div>
      ) : error ? (
        <div className="photo-dialog-stage"><ErrorNotice error={error} onRetry={onRetry} /></div>
      ) : photoUrl ? (
        <PhotoViewer key={`${photoUrl}:${counterText || ""}`} photoUrl={photoUrl} title={title} />
      ) : null}
      {caption && <p className="photo-dialog-caption">{caption}</p>}
      <div className="photo-dialog-nav">
        <div className="photo-dialog-pager">
          <button
            type="button"
            className="photo-viewer-control"
            disabled={!hasPrev || !onPrev}
            onClick={onPrev}
            aria-label="Foto sebelumnya"
            title="Foto sebelumnya"
          >
            <Icon name="previous" />
          </button>
          {counterText && (
            <span className="photo-dialog-counter" aria-live="polite">{counterText}</span>
          )}
          <button
            type="button"
            className="photo-viewer-control"
            disabled={!hasNext || !onNext}
            onClick={onNext}
            aria-label="Foto berikutnya"
            title="Foto berikutnya"
          >
            <Icon name="next" />
          </button>
        </div>
        {photoUrl && downloadName && (
          <a
            className="photo-dialog-download"
            href={photoUrl}
            download={downloadName}
          >
            <Icon name="download" size={18} />
            Unduh foto
          </a>
        )}
      </div>
    </Dialog>
  );
}
