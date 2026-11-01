import { useEffect, useState } from "react";
import { pushSupported, syncPush } from "../lib/push";
import { Dialog, ErrorNotice } from "./common";

export default function PushPrompt({ userId }) {
  const [open, setOpen] = useState(() => pushSupported() && Notification.permission === "default");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(null);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (!pushSupported()) return;
    let active = true;
    // Granted permission can restore delivery without opening a browser permission prompt.
    syncPush(userId, Notification.permission === "granted").catch(() => {
      if (active && Notification.permission === "granted") {
        setError("Notifikasi belum tersambung. Coba lagi saat membuka aplikasi berikutnya.");
        setOpen(true);
      }
    });
    return () => { active = false; };
  }, [userId]);

  async function enable() {
    setBusy(true);
    setError(null);
    try {
      const result = await syncPush(userId, true);
      if (result === "on" || result === "off") setOpen(false);
      else setMessage(result === "denied"
        ? "Notifikasi diblokir. Untuk mengizinkannya, ubah izin situs di pengaturan browser."
        : "Notifikasi belum tersedia di server ini.");
    } catch {
      setError("Notifikasi belum tersambung. Tutup pesan ini dan coba lagi saat membuka aplikasi berikutnya.");
    } finally {
      setBusy(false);
    }
  }

  if (!open) return null;
  return (
    <Dialog title="Notifikasi pesan baru" onClose={() => setOpen(false)} dismissible={!busy}>
      <p>{message || "Izinkan notifikasi agar pesan baru tetap terlihat saat aplikasi tidak dibuka."}</p>
      <ErrorNotice error={error} />
      <div className="dialog-actions">
        <button className="button button-outline" onClick={() => setOpen(false)} disabled={busy}>Nanti</button>
        {!message && !error && (
          <button className="button button-primary" onClick={enable} disabled={busy}>
            {busy ? "Mengaktifkan…" : "Izinkan notifikasi"}
          </button>
        )}
      </div>
    </Dialog>
  );
}
