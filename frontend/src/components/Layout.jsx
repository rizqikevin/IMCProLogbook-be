import { useState } from "react";
import { Link, NavLink, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../auth";
import { ErrorNotice, Icon } from "./common";

export default function Layout() {
  const { user, logout } = useAuth();
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
  const { pathname } = useLocation();
  const editing = pathname === "/new" || pathname.endsWith("/add");
  async function signOut() {
    setBusy(true);
    setError(null);
    try {
      await logout();
    } catch (error) {
      setError(error);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <a className="skip-link" href="#main">
        Lewati ke konten
      </a>
      <header className="app-header">
        <div className="header-inner">
          <Link className="wordmark" to="/">
            <strong>
              Machine Logbook<span className="brand-period">.</span>
            </strong>
            <span>Arsip catatan produksi</span>
          </Link>
          <nav aria-label="Navigasi utama" className="desktop-nav">
            <NavLink to="/" end>
              Arsip logbook
            </NavLink>
            <NavLink to="/new">Arsip baru</NavLink>
          </nav>
          <div className="account">
            <div className="account-name">
              <strong>{user.name}</strong>
              <span>
                {user.role === "admin" ? "Administrator" : "Operator"}
              </span>
            </div>
            {!editing && (
              <button
                className="icon-button"
                aria-label="Keluar dari akun"
                title="Keluar"
                onClick={signOut}
                disabled={busy}
              >
                <Icon name="logout" />
              </button>
            )}
          </div>
        </div>
      </header>
      {error && (
        <div className="page">
          <ErrorNotice error={error} />
        </div>
      )}
      <main id="main" tabIndex={-1}>
        <Outlet />
      </main>
      <footer className="app-footer">
        <span>Machine Logbook Archive</span>
        <span>Mesin · Tanggal · Shift</span>
      </footer>
    </>
  );
}
