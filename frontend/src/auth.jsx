import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useState,
} from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { advanceSession, getSessionGeneration, request } from "./lib/api";
import { queryClient } from "./queryClient";
import { ErrorNotice, SkeletonAuth } from "./components/common";
import { disablePush } from "./lib/push";

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null);
  const [status, setStatus] = useState("loading");
  const [error, setError] = useState(null);
  const [attempt, setAttempt] = useState(0);
  const clear = useCallback(() => {
    advanceSession();
    setUser(null);
    setError(null);
    setStatus("anonymous");
    queryClient.clear();
  }, []);

  useEffect(() => {
    const generation = getSessionGeneration();
    const controller = new AbortController();
    setStatus("loading");
    setError(null);
    request("/auth/me", { signal: controller.signal, authenticated: false })
      .then((data) => {
        if (controller.signal.aborted || generation !== getSessionGeneration()) return;
        setUser(data);
        setStatus("authenticated");
      })
      .catch((error) => {
        if (controller.signal.aborted || generation !== getSessionGeneration() || error.name === "AbortError") return;
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


  async function login(username, password) {
    const generation = advanceSession();
    await request("/auth/login", {
      method: "POST",
      body: { username, password },
      authenticated: false,
    });
    if (generation !== getSessionGeneration()) return;
    const identityGeneration = advanceSession();
    const identity = await request("/auth/me");
    if (identityGeneration !== getSessionGeneration()) return;
    queryClient.clear();
    setError(null);
    setUser(identity);
    setStatus("authenticated");
  }

  async function logout() {
    const generation = getSessionGeneration();
    await disablePush();
    if (generation !== getSessionGeneration()) return;
    await request("/auth/logout", { method: "POST", authenticated: false });
    if (generation === getSessionGeneration()) clear();
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
  if (status === "loading") return <SkeletonAuth />;
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
