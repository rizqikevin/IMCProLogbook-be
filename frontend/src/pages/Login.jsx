import { useState } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../auth";
import { ErrorNotice, Icon, Loading } from "../components/common";

export default function Login() {
  const { login, status, error: sessionError, retry } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [visible, setVisible] = useState(false);
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const from = location.state?.from;
  const destination =
    typeof from === "string" &&
    from.startsWith("/") &&
    !from.startsWith("//") &&
    from !== "/login"
      ? from
      : "/";
  if (status === "loading") return <Loading full>Memeriksa sesi Anda…</Loading>;
  if (status === "authenticated") return <Navigate to={destination} replace />;
  if (status === "error")
    return (
      <main className="standalone">
        <ErrorNotice error={sessionError} onRetry={retry} />
      </main>
    );
  async function submit(event) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError(null);
    setBusy(true);
    try {
      await login(data.get("username").trim(), data.get("password"));
      navigate(destination, { replace: true });
    } catch (error) {
      setError(error);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="login-page">
      <section className="login-story">
        <div className="wordmark">
          <strong>
            Machine Logbook<span className="brand-period">.</span>
          </strong>
          <span>Arsip catatan produksi</span>
        </div>
        <div className="login-title">
          <p className="section-label">
            Dari lantai produksi, untuk setiap shift.
          </p>
          <h1>
            Setiap halaman.
            <br />
            Satu tempat.
          </h1>
          <p>
            Simpan foto logbook mesin dan temukan kembali catatan yang Anda
            butuhkan.
          </p>
        </div>
        <div className="login-register">
          <span>
            01 <strong>Pilih mesin</strong>
          </span>
          <span>
            02 <strong>Foto halamannya</strong>
          </span>
          <span>
            03 <strong>Simpan arsip</strong>
          </span>
        </div>
        <p className="login-footnote">Machine Logbook Archive</p>
      </section>
      <section className="login-form-panel">
        <div className="login-form-wrap">
          <p className="section-label">Akses operator</p>
          <h2>Selamat datang kembali.</h2>
          <p className="muted">Masuk untuk melihat dan mengarsipkan logbook.</p>
          <form onSubmit={submit} className="login-form">
            <ErrorNotice error={error} />
            <label>
              Nama pengguna
              <input
                name="username"
                autoComplete="username"
                required
                maxLength={64}
                autoCapitalize="none"
                spellCheck={false}
                placeholder="Nama pengguna Anda"
                disabled={busy}
              />
            </label>
            <label>
              Kata sandi
              <div className="password-input">
                <input
                  name="password"
                  type={visible ? "text" : "password"}
                  autoComplete="current-password"
                  required
                  maxLength={72}
                  placeholder="Kata sandi Anda"
                  disabled={busy}
                />
                <button
                  type="button"
                  className="icon-button"
                  aria-label={
                    visible ? "Sembunyikan kata sandi" : "Tampilkan kata sandi"
                  }
                  aria-pressed={visible}
                  onClick={() => setVisible(!visible)}
                >
                  <Icon name="eye" />
                </button>
              </div>
            </label>
            <button
              className="button button-primary button-wide"
              type="submit"
              disabled={busy}
            >
              {busy ? "Memeriksa akun…" : "Masuk ke arsip"}
            </button>
          </form>
          <p className="login-help">
            Belum memiliki akun atau lupa kata sandi?
            <br />
            Hubungi administrator untuk bantuan akses.
          </p>
        </div>
        <p className="login-bottom">Catatan tersusun. Pekerjaan berlanjut.</p>
      </section>
    </main>
  );
}
