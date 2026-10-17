import { useEffect, useRef, useState } from "react";
import { usePhotoUrl } from "../hooks";
import { ErrorNotice, Icon, Loading } from "./common";
import PhotoDialog from "./PhotoDialog";

export default function MessagePhoto({ photo, onOpen }) {
  const containerRef = useRef(null);
  const [isVisible, setIsVisible] = useState(
    () => typeof IntersectionObserver === "undefined",
  );
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (isVisible) return;
    const element = containerRef.current;
    if (!element || typeof IntersectionObserver === "undefined") {
      setIsVisible(true);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const [entry] = entries;
        if (entry && (entry.isIntersecting || entry.intersectionRatio > 0)) {
          setIsVisible(true);
          observer.disconnect();
        }
      },
      { rootMargin: "300px 0px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [isVisible]);

  const { url, loading, error, retry } = usePhotoUrl(
    isVisible ? photo.url : null,
  );

  return (
    <div ref={containerRef} className="message-photo">
      {error ? (
        <div className="message-photo-error">
          <ErrorNotice error={error} onRetry={retry} />
        </div>
      ) : (
        <button
          type="button"
          className="message-photo-open"
          onFocus={() => setIsVisible(true)}
          onClick={() => {
            setIsVisible(true);
            if (onOpen) onOpen(photo);
            else setOpen(true);
          }}
          aria-label={`Perbesar foto ${photo.page_number}`}
        >
          {!isVisible ? (
            <div className="message-photo-placeholder" aria-hidden="true">
              <Icon name="camera" size={24} />
            </div>
          ) : loading || !url ? (
            <div
              className="message-photo-placeholder"
              aria-label={`Memuat foto ${photo.page_number}…`}
            >
              <Loading>Memuat foto {photo.page_number}…</Loading>
            </div>
          ) : (
            <img
              src={url}
              alt={`Foto ${photo.page_number}`}
              loading="lazy"
              decoding="async"
            />
          )}
          <span>Foto {photo.page_number} · Perbesar</span>
        </button>
      )}
      {!onOpen && open && (
        <PhotoDialog
          title={`Foto ${photo.page_number}`}
          photoUrl={url}
          counterText={`Foto ${photo.page_number}`}
          downloadName={`foto-${photo.id}.${photo.content_type?.split("/")[1] || "jpg"}`}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}
