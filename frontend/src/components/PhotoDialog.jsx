import { useRef, useState, useEffect } from "react";
import {
  Dialog,
  ErrorNotice,
  Icon,
  Loading,
  SkeletonPhotoDialog,
} from "./common";
import { usePhotoMarkers } from "../hooks";

const MAX_ZOOM = 4;

function PhotoViewer({ photoUrl, title, onClose, markers = [], onAddMarker }) {
  const stageRef = useRef(null);
  const imageRef = useRef(null);
  const pointers = useRef(new Map());
  const viewRef = useRef({ scale: 1, x: 0, y: 0 });
  const [view, setView] = useState(viewRef.current);
  const [rotation, setRotation] = useState(0);
  const [markerMode, setMarkerMode] = useState(false);
  const rotationRef = useRef(0);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [imageSize, setImageSize] = useState({ width: 0, height: 0 });
  const gesture = useRef(null);
  const [markerCursor, setMarkerCursor] = useState({ x: 0.5, y: 0.5 });

  function updateView(next, nextRotation = rotationRef.current) {
    const stage = stageRef.current;
    const image = imageRef.current;
    if (!stage || !image || !image.naturalWidth) return;
    const fit = Math.min(1, stage.clientWidth / image.naturalWidth, stage.clientHeight / image.naturalHeight);
    const width = image.naturalWidth * fit;
    const height = image.naturalHeight * fit;
    setImageSize({ width, height });
    const isTransposed = nextRotation % 180 !== 0;
    const w = isTransposed ? height : width;
    const h = isTransposed ? width : height;
    const scale = Math.min(MAX_ZOOM, Math.max(1, next.scale));
    const maxX = Math.max(0, (w * scale - stage.clientWidth) / 2);
    const maxY = Math.max(0, (h * scale - stage.clientHeight) / 2);
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

  function rotate(direction = 1) {
    const next = (rotationRef.current + direction * 90 + 360) % 360;
    rotationRef.current = next;
    setRotation(next);
    updateView(viewRef.current, next);
  }

  function reset() {
    if (view.scale !== 1 || view.x !== 0 || view.y !== 0) {
      updateView({ scale: 1, x: 0, y: 0 });
    } else if (rotationRef.current !== 0) {
      rotationRef.current = 0;
      setRotation(0);
      updateView({ scale: 1, x: 0, y: 0 }, 0);
    }
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
    if (pointers.current.size) {
      if (gesture.current) gesture.current.moved = true;
    } else {
      gesture.current = { x: event.clientX, y: event.clientY, moved: false };
    }
    pointers.current.set(event.pointerId, {
      x: event.clientX,
      y: event.clientY,
    });
    setDragging(true);
  }

  function pointerMove(event) {
    const points = pointers.current;
    const previous = points.get(event.pointerId);
    if (!previous) return;
    if (gesture.current && Math.hypot(event.clientX - gesture.current.x, event.clientY - gesture.current.y) > 5) gesture.current.moved = true;
    const current = { x: event.clientX, y: event.clientY };
    const other = [...points.entries()].find(
      ([id]) => id !== event.pointerId,
    )?.[1];
    const old = viewRef.current;
    if (other) {
      const before = Math.hypot(previous.x - other.x, previous.y - other.y);
      const after = Math.hypot(current.x - other.x, current.y - other.y);
      const scale = Math.min(
        MAX_ZOOM,
        Math.max(1, old.scale * (before ? after / before : 1)),
      );
      const rect = event.currentTarget.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;
      const ratio = scale / old.scale;
      updateView({
        scale,
        x:
          (current.x + other.x) / 2 -
          centerX -
          ((previous.x + other.x) / 2 - centerX - old.x) * ratio,
        y:
          (current.y + other.y) / 2 -
          centerY -
          ((previous.y + other.y) / 2 - centerY - old.y) * ratio,
      });
    } else {
      updateView({
        ...old,
        x: old.x + current.x - previous.x,
        y: old.y + current.y - previous.y,
      });
    }
    points.set(event.pointerId, current);
  }

  function pointerEnd(event) {
    if (event.type === "pointerup" && pointers.current.size === 1 && !gesture.current?.moved) toggleMarker(event);
    pointers.current.delete(event.pointerId);
    setDragging(pointers.current.size > 0);
  }

  function handleKeyDown(event) {
    if (!ready) return;
    if (markerMode && onAddMarker && !event.shiftKey) {
      const directions = { ArrowLeft: [-0.02, 0], ArrowRight: [0.02, 0], ArrowUp: [0, -0.02], ArrowDown: [0, 0.02] };
      if (directions[event.key] || event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        event.stopPropagation();
        if (directions[event.key]) {
          const [dx, dy] = directions[event.key];
          const delta = new DOMMatrixReadOnly().rotate(-rotationRef.current).transformPoint(new DOMPoint(dx, dy, 0, 0));
          setMarkerCursor((point) => ({ x: Math.max(0, Math.min(1, point.x + delta.x)), y: Math.max(0, Math.min(1, point.y + delta.y)) }));
        } else onAddMarker(markerCursor);
        return;
      }
    }
    const steps = {
      ArrowLeft: [40, 0],
      ArrowRight: [-40, 0],
      ArrowUp: [0, 40],
      ArrowDown: [0, -40],
    };
    if (event.shiftKey && steps[event.key]) {
      event.preventDefault();
      event.stopPropagation();
      const [x, y] = steps[event.key];
      updateView({
        ...viewRef.current,
        x: viewRef.current.x + x,
        y: viewRef.current.y + y,
      });
    }
  }

  useEffect(() => {
    function onKeyDown(event) {
      if (!ready) return;
      if (
        event.target.tagName === "INPUT" ||
        event.target.tagName === "TEXTAREA"
      )
        return;
      if (
        (event.key === "r" || event.key === "R") &&
        !event.ctrlKey &&
        !event.metaKey &&
        !event.altKey
      ) {
        event.preventDefault();
        rotate(event.shiftKey ? -1 : 1);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [ready]);
  function toggleMarker(event) {
    if (!markerMode || !ready || !onAddMarker || event.target.closest("button")) return;
    const image = imageRef.current;
    const rect = image.getBoundingClientRect();
    if (!rect.width || !rect.height) return;
    const matrix = new DOMMatrixReadOnly(getComputedStyle(image.parentElement).transform).inverse();
    const point = matrix.transformPoint(new DOMPoint(event.clientX - rect.left - rect.width / 2, event.clientY - rect.top - rect.height / 2, 0, 0));
    const marker = { x: point.x / imageSize.width + 0.5, y: point.y / imageSize.height + 0.5 };
    if (marker.x >= 0 && marker.x <= 1 && marker.y >= 0 && marker.y <= 1) onAddMarker(marker);
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
        <button type="button" className="photo-dialog-close" aria-label="Tutup foto" onPointerDown={(event) => event.stopPropagation()} onClick={onClose}>
          <Icon name="close" />
        </button>
        <div
          className="photo-dialog-image-layer"
          style={{
            width: imageSize.width || undefined,
            height: imageSize.height || undefined,
            transform: `translate(${view.x}px, ${view.y}px) rotate(${rotation}deg) scale(${view.scale})`,
            transition: dragging ? "none" : "transform 0.15s ease",
            visibility: ready ? "visible" : "hidden",
          }}
        >
          <img
            ref={imageRef}
            src={photoUrl}
            alt={title}
            draggable={false}
            onLoad={() => {
              setReady(true);
              updateView(viewRef.current);
            }}
            onError={() => {
              setFailed(true);
              setReady(false);
            }}
          />
          <div className="photo-dialog-markers" aria-hidden="true">
            {markers.map((marker, index) => <span key={`${marker.x}:${marker.y}:${index}`} className="photo-dialog-marker" style={{ left: `${marker.x * 100}%`, top: `${marker.y * 100}%` }} />)}
            {markerMode && onAddMarker && <span className="photo-dialog-marker-cursor" style={{ left: `${markerCursor.x * 100}%`, top: `${markerCursor.y * 100}%` }} />}
          </div>
        </div>
        {!ready && (
          <div className="photo-dialog-image-status">
            {failed ? (
              "Foto tidak dapat ditampilkan."
            ) : (
              <Loading>Memuat foto…</Loading>
            )}
          </div>
        )}
      </div>
      <div
        className="photo-dialog-zoom"
        role="group"
        aria-label="Ukuran dan rotasi foto"
      >
        <button
          type="button"
          className="photo-viewer-control"
          aria-label="Perkecil foto"
          title="Perkecil foto"
          disabled={!ready || view.scale <= 1}
          onClick={() => zoom(view.scale - 0.5)}
        >
          <Icon name="minus" />
        </button>
        <output aria-live="polite" aria-label="Pembesaran foto">
          {Math.round(view.scale * 100)}%
        </output>
        <button
          type="button"
          className="photo-viewer-control"
          aria-label="Perbesar foto"
          title="Perbesar foto"
          disabled={!ready || view.scale >= MAX_ZOOM}
          onClick={() => zoom(view.scale + 0.5)}
        >
          <Icon name="plus" />
        </button>
        <span className="photo-dialog-tool-divider" aria-hidden="true" />
        <button
          type="button"
          className="photo-viewer-control photo-dialog-reset"
          aria-label="Atur ulang ukuran dan rotasi foto"
          title={
            view.scale !== 1
              ? "Atur ulang ukuran foto (100%)"
              : rotation !== 0
                ? "Atur ulang rotasi (0°)"
                : "Atur ulang"
          }
          disabled={
            !ready ||
            (view.scale === 1 && view.x === 0 && view.y === 0 && rotation === 0)
          }
          onClick={reset}
        >
          <Icon name="fit" />
        </button>
        <span className="photo-dialog-tool-divider" aria-hidden="true" />
        <button
          type="button"
          className={`photo-viewer-control${markerMode ? " is-active" : ""}`}
          aria-label={markerMode ? "Nonaktifkan marker foto" : "Aktifkan marker foto"}
          aria-pressed={markerMode}
          title="Tandai foto"
          disabled={!ready || !onAddMarker}
          onClick={() => { setMarkerMode((active) => !active); stageRef.current?.focus(); }}
        >
          <Icon name="marker" />
        </button>
        <span className="photo-dialog-tool-divider" aria-hidden="true" />
        <button
          type="button"
          className="photo-viewer-control"
          aria-label={
            rotation
              ? `Putar foto 90 derajat searah jarum jam (saat ini ${rotation}°)`
              : "Putar foto 90 derajat searah jarum jam"
          }
          title="Putar foto 90° (R)"
          disabled={!ready}
          onClick={(event) => rotate(event.shiftKey ? -1 : 1)}
        >
          <Icon name="rotate" />
        </button>
      </div>
      <p className="photo-dialog-hint">
        Cubit untuk zoom · Putar foto (R) · Geser saat diperbesar
        <span> · Shift + panah untuk menggeser</span>
      </p>
      {markerMode && <p className="photo-dialog-marker-help">Klik atau ketuk foto untuk menambah titik. Panah mengatur posisi, Enter menyimpan. Geser untuk pan.</p>}
    </>
  );
}

export default function PhotoDialog({
  title, subtitle, caption, photoUrl, loading = false, error = null, onRetry,
  downloadName, onPrev, onNext, hasPrev = false, hasNext = false, counterText,
  onClose, markerPhotoUrl,
}) {
  const markerQuery = usePhotoMarkers(markerPhotoUrl);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const historyEntry = useRef(null);

  useEffect(() => {
    if (!window.matchMedia("(max-width: 700px)").matches) return;
    let cancelled = false;
    const marker = `photo-${Date.now()}`;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    // Defer history mutation so StrictMode's discarded effect adds no entry.
    queueMicrotask(() => {
      if (cancelled) return;
      historyEntry.current = marker;
      window.history.pushState(
        { ...window.history.state, photoViewer: marker },
        "",
      );
    });
    function handleBack() {
      historyEntry.current = null;
      closeRef.current();
    }
    window.addEventListener("popstate", handleBack);
    return () => {
      cancelled = true;
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("popstate", handleBack);
      if (
        historyEntry.current === marker &&
        window.history.state?.photoViewer === marker
      ) {
        historyEntry.current = null;
        window.history.back();
      }
    };
  }, []);

  function closeViewer() {
    if (
      historyEntry.current &&
      window.history.state?.photoViewer === historyEntry.current
    ) {
      window.history.back();
    } else {
      onClose();
    }
  }
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
    <Dialog title={title} className="photo-dialog" onClose={closeViewer}>
      <p className="photo-dialog-exit-hint">
        Gunakan tombol Kembali atau Escape untuk menutup foto.
      </p>
      {subtitle && <p className="photo-dialog-subtitle">{subtitle}</p>}
      {loading ? (
        <SkeletonPhotoDialog />
      ) : error ? (
        <div className="photo-dialog-stage">
          <ErrorNotice error={error} onRetry={onRetry} />
        </div>
      ) : photoUrl ? (
        <PhotoViewer
          key={`${photoUrl}:${counterText || ""}`}
          photoUrl={photoUrl}
          title={title}
          onClose={closeViewer}
          markers={markerQuery.markers}
          onAddMarker={markerQuery.canEdit && !markerQuery.saving ? markerQuery.add : undefined}
        />
      ) : null}
      {markerQuery.loading && <Loading>Memuat marker…</Loading>}
      {markerQuery.saving && <p role="status">Menyimpan marker…</p>}
      <ErrorNotice error={markerQuery.error} onRetry={markerQuery.retry} />
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
            <span className="photo-dialog-counter" aria-live="polite">
              {counterText}
            </span>
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
