import { useEffect, useState } from "react";
import { requestPhoto } from "../lib/api";
import { Dialog, ErrorNotice, Loading } from "./common";

export default function MessagePhoto({ photo }) {
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
        <button type="button" className="message-photo-open" onClick={() => setOpen(true)} aria-label={`Perbesar foto ${photo.page_number}`}>
          <img src={state.url} alt={`Foto ${photo.page_number}`} loading="lazy" />
          <span>Foto {photo.page_number} · Perbesar</span>
        </button>}
      {open && <Dialog title={`Foto ${photo.page_number}`} className="photo-dialog" onClose={() => setOpen(false)}>
        <img src={state.url} alt={`Foto ${photo.page_number}`} />
        <a className="button button-outline" href={state.url} download={`foto-${photo.id}.${photo.content_type.split("/")[1]}`}>Unduh foto</a>
      </Dialog>}
    </div>
  );
}
