import { useEffect, useState } from "react";
import { useAuth } from "../auth";
import { disablePush, pushSupported, syncPush } from "../lib/push";
import { ErrorNotice } from "./common";

const descriptions = {
  loading: "Memeriksa notifikasi…",
  unsupported: "Notifikasi tidak didukung di browser ini. Gunakan HTTPS; di iPhone/iPad, pasang aplikasi ke Layar Utama lalu buka dari sana.",
  disabled: "Notifikasi belum diaktifkan oleh administrator server.",
  denied: "Izin notifikasi diblokir. Izinkan lewat pengaturan situs/browser, lalu periksa lagi.",
  off: "Notifikasi mati di browser ini. Aktifkan untuk menerima pesan baru dari mesin yang dapat Anda akses, termasuk saat aplikasi ditutup.",
  on: "Notifikasi aktif untuk sesi akun ini. Isi pesan tidak ditampilkan. Pengiriman mengikuti izin dan pengaturan browser/perangkat.",
  error: "Pengaturan notifikasi belum tersimpan. Periksa koneksi lalu coba lagi.",
};

export default function NotificationSettings({ hidden = false }) {
  const { user } = useAuth();
  const [state, setState] = useState("loading");
  const [error, setError] = useState(null);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    setError(null);
    if (!pushSupported()) { setState("unsupported"); return; }
    setState("loading");
    syncPush(user.id).then((value) => { if (active) setState(value); }).catch((failure) => {
      if (active) { setError(failure); setState("error"); }
    });
    return () => { active = false; };
  }, [user.id, attempt]);

  async function toggle() {
    const disable = state === "on";
    setError(null);
    setState("loading");
    try {
      if (disable) { await disablePush(); setState("off"); }
      else setState(await syncPush(user.id, true));
    } catch (failure) { setError(failure); setState("error"); }
  }

  return (
    <section className="page" hidden={hidden} aria-labelledby="notification-heading">
      <h2 id="notification-heading">Notifikasi pesan</h2>
      <p id="notification-status" role="status">{descriptions[state]}</p>
      <ErrorNotice error={error} />
      {(state === "on" || state === "off") && <button type="button" className="button" onClick={toggle} aria-describedby="notification-status">
        {state === "on" ? "Matikan notifikasi" : "Aktifkan notifikasi"}
      </button>}
      {(state === "error" || state === "denied" || state === "disabled") && <button type="button" className="button" onClick={() => setAttempt((value) => value + 1)}>Periksa lagi</button>}
    </section>
  );
}
