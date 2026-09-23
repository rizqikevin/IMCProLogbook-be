import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { readSession, request, saveSession } from "./lib/api";
import { ErrorNotice, Loading } from "./components/common";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState("loading");
  const [error, setError] = useState(null);
  const [attempt, setAttempt] = useState(0);
  const [expiresAt, setExpiresAt] = useState(() => readSession()?.expires_at);
  const clear = useCallback(() => {
    saveSession(null);
    setUser(null);
    setExpiresAt(null);
    setStatus("anonymous");
  }, []);

  useEffect(() => {
    const session = readSession();
    if (!session || new Date(session.expires_at).getTime() <= Date.now()) {
      clear();
      return;
    }
    const controller = new AbortController();
    setStatus("loading");
    setError(null);
    request("/auth/me", { signal: controller.signal })
      .then((data) => {
        setUser(data);
        setStatus("authenticated");
      })
      .catch((error) => {
        if (error.name === "AbortError") return;
        if (error.status === 401) clear();
        else {
          setError(error);
          setStatus("error");
        }
      });
    return () => controller.abort();
  }, [attempt, clear]);

  useEffect(() => {
    window.addEventListener("logbook:session-expired", clear);
    return () => window.removeEventListener("logbook:session-expired", clear);
  }, [clear]);

  useEffect(() => {
    if (!expiresAt) return;
    const remaining = new Date(expiresAt).getTime() - Date.now();
    const timer = setTimeout(
      clear,
      Math.max(0, Math.min(remaining, 2_147_483_647)),
    );
    return () => clearTimeout(timer);
  }, [expiresAt, clear]);

  async function login(username, password) {
    const result = await request("/auth/login", {
      method: "POST",
      body: { username, password },
      authenticated: false,
    });
    saveSession({
      access_token: result.access_token,
      expires_at: result.expires_at,
    });
    setExpiresAt(result.expires_at);
    setUser(result.user);
    setStatus("authenticated");
  }

  async function logout() {
    try {
      await request("/auth/logout", { method: "POST" });
    } catch (error) {
      if (error.status !== 401) throw error;
    }
    clear();
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        status,
        error,
        login,
        logout,
        retry: () => setAttempt((value) => value + 1),
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}

export function RequireAuth() {
  const { status, error, retry } = useAuth();
  const location = useLocation();
  if (status === "loading") return <Loading full>Memeriksa sesi Anda…</Loading>;
  if (status === "error")
    return (
      <main className="standalone">
        <h1>Koneksi belum tersedia</h1>
        <ErrorNotice error={error} onRetry={retry} />
      </main>
    );
  if (status !== "authenticated")
    return (
      <Navigate
        to="/login"
        state={{ from: location.pathname + location.search }}
        replace
      />
    );
  return <Outlet />;
}
