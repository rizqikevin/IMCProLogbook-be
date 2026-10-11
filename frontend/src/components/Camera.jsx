import { useEffect, useRef, useState } from "react";
import { Dialog, ErrorNotice, Icon, Loading } from "./common";

export default function Camera({ onClose, onCapture, photos, maximum }) {
  const count = photos.length;
  const video = useRef(null);
  const thumbnails = useRef(null);
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
          ? "Izin kamera ditolak. Izinkan akses kamera di pengaturan peramban."
          : err.message || "Gagal membuka kamera.",
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
          video: { facingMode: facing },
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
    if (!ready || busy || count >= maximum) return;
    setBusy(true);
    setError(null);
    try {
      const canvas = document.createElement("canvas");
      const ratio = Math.min(
        1,
        2560 / Math.max(video.current.videoWidth, video.current.videoHeight),
      );
      canvas.width = Math.round(video.current.videoWidth * ratio);
      canvas.height = Math.round(video.current.videoHeight * ratio);
      if (!canvas.width || !canvas.height)
        throw new Error("Tunggu sampai tampilan kamera siap.");
      canvas
        .getContext("2d")
        .drawImage(video.current, 0, 0, canvas.width, canvas.height);
      const blob = await new Promise((resolve) =>
        canvas.toBlob(resolve, "image/jpeg", 0.94),
      );
      if (!blob)
        throw new Error("Foto belum berhasil diambil. Silakan coba lagi.");
      await onCapture([
        new File([blob], `logbook-${Date.now()}.jpg`, { type: "image/jpeg" }),
      ]);
    } catch (error) {
      setError(error.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      title="Foto halaman logbook"
      className="camera-dialog"
      onClose={onClose}
      dismissible={!busy}
    >
      <p className="muted">
        Pastikan seluruh halaman masuk bingkai dan tulisan terbaca.
      </p>
      <ErrorNotice
        error={error}
        onRetry={() => setAttempt((value) => value + 1)}
      />
      <div className="camera-preview">
        <video ref={video} muted playsInline aria-label="Pratinjau kamera" />
        {!ready && !error && <Loading>Membuka kamera…</Loading>}
      </div>
      <div className="camera-status" role="status">
        <span>{count} foto dipilih</span>
        <span>Belum diunggah</span>
      </div>
      {count > 0 && (
        <ol
          className="camera-thumbnails"
          aria-label="Foto yang sudah dipilih"
          ref={thumbnails}
          tabIndex={0}
        >
          {photos.map((photo, index) => (
            <li key={photo.url}>
              <img src={photo.url} alt={`Foto pilihan ${index + 1}`} />
              <span>Foto {index + 1}</span>
            </li>
          ))}
        </ol>
      )}
      <div className="camera-controls">
        <button
          type="button"
          className="button button-outline"
          onClick={() =>
            setFacing((value) =>
              value === "environment" ? "user" : "environment",
            )
          }
          disabled={busy}
        >
          Ganti kamera
        </button>
        <button
          type="button"
          className="button button-primary"
          onClick={capture}
          disabled={!ready || busy || count >= maximum}
        >
          <Icon name="camera" />
          {busy
            ? "Menyimpan foto…"
            : count >= maximum
              ? "Batas foto tercapai"
              : "Ambil foto"}
        </button>
        <button
          type="button"
          className="button button-outline"
          onClick={onClose}
          disabled={busy}
        >
          Selesai memotret
        </button>
      </div>
    </Dialog>
  );
}
