import { useState } from "react";
import { Link, NavLink, Outlet, useMatch } from "react-router-dom";
import { useAuth } from "../auth";
import { ErrorNotice, Icon } from "./common";
import { MessageDrafts } from "./MessageDrafts";

export default function Layout() {
  const { user, logout } = useAuth();
  const conversation = useMatch("/machines/:machineId");
  const [error, setError] = useState(null);
  const [busy, setBusy] = useState(false);
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
      {!conversation && <header className="app-header">
        <div className="header-inner">
          <Link className="wordmark" to="/">
            <img
              className="brand-logo"
              src="/brand/imcpro.png"
              alt="IMCPro"
              width="962"
              height="213"
            />
            <span>Machine Logbook Archive</span>
          </Link>
          <nav aria-label="Navigasi utama" className="desktop-nav">
            <NavLink to="/" end>
              Pilih mesin
            </NavLink>
          </nav>
          <div className="account">
            <div className="account-name">
              <strong>{user.name}</strong>
              <span>
                {user.role === "admin" ? "Administrator" : "Operator"}
              </span>
            </div>
              <button
                className="icon-button"
                aria-label="Keluar dari akun"
                title="Keluar"
                onClick={signOut}
                disabled={busy}
              >
                <Icon name="logout" />
              </button>
          </div>
        </div>
      </header>}
      {error && (
        <div className="page">
          <ErrorNotice error={error} />
        </div>
      )}
      <main id="main" tabIndex={-1} className={conversation ? "conversation-main" : undefined}>
        <MessageDrafts><Outlet /></MessageDrafts>
      </main>
      {!conversation && <footer className="app-footer">
        <span>Machine Logbook Archive</span>
        <span>Catatan dan foto mesin</span>
      </footer>}
    </>
  );
}
