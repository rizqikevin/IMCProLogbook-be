import { useEffect, useRef, useState } from "react";
import { Dialog, ErrorNotice, Icon, Loading } from "./common";

export default function Camera({ onClose, onCapture, count, maximum }) {
  const video = useRef(null);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const [facing, setFacing] = useState("environment");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let disposed = false;
    let stream;
    setReady(false);
    setError(null);
    async function start() {
      try {
        if (!navigator.mediaDevices?.getUserMedia)
          throw new Error(
            "Kamera langsung memerlukan HTTPS atau localhost. Anda tetap dapat memilih foto dari perangkat.",
          );
        stream = await navigator.mediaDevices.getUserMedia({
          video: {
            facingMode: { ideal: facing },
            width: { ideal: 2560 },
            height: { ideal: 1920 },
          },
          audio: false,
        });
        if (disposed) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }
        video.current.srcObject = stream;
        await video.current.play();
        if (!disposed) setReady(true);
      } catch (error) {
        if (disposed) return;
        const messages = {
          NotAllowedError:
            "Izin kamera belum diberikan. Izinkan kamera di pengaturan browser, atau pilih foto dari perangkat.",
          NotFoundError: "Kamera tidak ditemukan. Pilih foto dari perangkat.",
          NotReadableError:
            "Kamera sedang digunakan aplikasi lain. Tutup aplikasi tersebut lalu coba lagi.",
        };
        setError(
          messages[error.name] || error.message || "Kamera belum dapat dibuka.",
        );
      }
    }
    start();
    return () => {
      disposed = true;
      stream?.getTracks().forEach((track) => track.stop());
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
      <div className="camera-controls">
        <button
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
