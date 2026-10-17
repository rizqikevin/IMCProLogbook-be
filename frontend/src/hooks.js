import { useEffect, useState } from "react";
import { request, requestPhoto } from "./lib/api";

export function useMachines() {
  const [state, setState] = useState({ machines: [], loading: true, error: null });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setState({ machines: [], loading: true, error: null });
    request("/machines", { signal: controller.signal })
      .then((page) => {
        if (!controller.signal.aborted)
          setState({ machines: page.items, loading: false, error: null });
      })
      .catch((error) => {
        if (error.name !== "AbortError")
          setState({ machines: [], loading: false, error });
      });
    return () => controller.abort();
  }, [attempt]);
  return { ...state, retry: () => setAttempt((value) => value + 1) };
}

export function useCatalog() {
  const [state, setState] = useState({
    machines: [],
    shifts: [],
    loading: true,
    error: null,
  });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setState((old) => ({ ...old, loading: true, error: null }));
    Promise.all([
      request("/machines", { signal: controller.signal }),
      request("/shifts", { signal: controller.signal }),
    ])
      .then(([machines, shifts]) =>
        setState({
          machines: machines.items,
          shifts: shifts.items,
          loading: false,
          error: null,
        }),
      )
      .catch((error) => {
        if (error.name !== "AbortError")
          setState((old) => ({ ...old, loading: false, error }));
      });
    return () => controller.abort();
  }, [attempt]);
  return { ...state, retry: () => setAttempt((value) => value + 1) };
}

export function useBook(id) {
  const [state, setState] = useState({
    book: null,
    loading: Boolean(id),
    error: null,
  });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (!id) return;
    const controller = new AbortController();
    setState({ book: null, loading: true, error: null });
    request(`/logbooks/${encodeURIComponent(id)}`, {
      signal: controller.signal,
    })
      .then((book) => setState({ book, loading: false, error: null }))
      .catch((error) => {
        if (error.name !== "AbortError")
          setState({ book: null, loading: false, error });
      });
    return () => controller.abort();
  }, [id, attempt]);
  return { ...state, retry: () => setAttempt((value) => value + 1) };
}

export function usePhoto(bookId, photoId) {
  const [state, setState] = useState({ url: "", loading: true, error: null });
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    if (!bookId || !photoId) return;
    const controller = new AbortController();
    let url = "";
    setState({ url: "", loading: true, error: null });
    request(
      `/logbooks/${encodeURIComponent(bookId)}/photos/${encodeURIComponent(photoId)}`,
      { signal: controller.signal, blob: true },
    )
      .then((blob) => {
        if (controller.signal.aborted) return;
        url = URL.createObjectURL(blob);
        setState({ url, loading: false, error: null });
      })
      .catch((error) => {
        if (error.name !== "AbortError")
          setState({ url: "", loading: false, error });
      });
    return () => {
      controller.abort();
      if (url) URL.revokeObjectURL(url);
    };
  }, [bookId, photoId, attempt]);
  return { ...state, retry: () => setAttempt((value) => value + 1) };
}

const photoBlobCache = new Map();

export function usePhotoUrl(url) {
  const [state, setState] = useState(() => ({
    url: photoBlobCache.has(url) ? photoBlobCache.get(url) : "",
    loading: Boolean(url) && !photoBlobCache.has(url),
    error: null,
  }));
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!url) {
      setState({ url: "", loading: false, error: null });
      return;
    }
    if (photoBlobCache.has(url)) {
      setState({ url: photoBlobCache.get(url), loading: false, error: null });
      return;
    }
    const controller = new AbortController();
    setState({ url: "", loading: true, error: null });
    Promise.resolve()
      .then(() => requestPhoto(url, controller.signal))
      .then((blob) => {
        if (controller.signal.aborted) return;
        if (photoBlobCache.size >= 50) {
          const oldestKey = photoBlobCache.keys().next().value;
          URL.revokeObjectURL(photoBlobCache.get(oldestKey));
          photoBlobCache.delete(oldestKey);
        }
        const objectUrl = URL.createObjectURL(blob);
        photoBlobCache.set(url, objectUrl);
        setState({ url: objectUrl, loading: false, error: null });
      })
      .catch((error) => {
        if (error.name !== "AbortError") {
          setState({ url: "", loading: false, error });
        }
      });
    return () => controller.abort();
  }, [url, attempt]);

  return { ...state, retry: () => setAttempt((value) => value + 1) };
}
