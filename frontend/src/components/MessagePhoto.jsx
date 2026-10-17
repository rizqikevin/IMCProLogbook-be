import { useEffect, useState } from "react";
import { requestPhoto } from "../lib/api";
import { ErrorNotice, Loading } from "./common";
import PhotoDialog from "./PhotoDialog";

export default function MessagePhoto({ photo, onOpen }) {
  const [state, setState] = useState({ loading: true, url: "", error: null });
  const [attempt, setAttempt] = useState(0);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    let url = "";
    setState({ loading: true, url: "", error: null });
    Promise.resolve().then(() => requestPhoto(photo.url, controller.signal))
      .then((blob) => {
        if (controller.signal.aborted) return;
        url = URL.createObjectURL(blob);
        setState({ loading: false, url, error: null });
      })
      .catch((error) => {
        if (error.name !== "AbortError") setState({ loading: false, url: "", error });
      });
    return () => { controller.abort(); if (url) URL.revokeObjectURL(url); };
  }, [photo.url, attempt]);
  return (
    <div className="message-photo">
      {state.loading ? <Loading>Memuat foto {photo.page_number}…</Loading> : state.error ?
        <ErrorNotice error={state.error} onRetry={() => setAttempt((value) => value + 1)} /> :
        <button
          type="button"
          className="message-photo-open"
          onClick={() => {
            if (onOpen) onOpen(photo);
            else setOpen(true);
          }}
          aria-label={`Perbesar foto ${photo.page_number}`}
        >
          <img src={state.url} alt={`Foto ${photo.page_number}`} loading="lazy" />
          <span>Foto {photo.page_number} · Perbesar</span>
        </button>}
      {!onOpen && open && (
        <PhotoDialog
          title={`Foto ${photo.page_number}`}
          photoUrl={state.url}
          counterText={`Foto ${photo.page_number}`}
          downloadName={`foto-${photo.id}.${photo.content_type?.split("/")[1] || "jpg"}`}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}
