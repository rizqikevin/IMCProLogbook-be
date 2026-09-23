import { useEffect, useRef } from "react";

export function Icon({ name, size = 20 }) {
  const paths = {
    camera: (
      <>
        <path d="M8 5 9.5 3h5L16 5h4v15H4V5Z" />
        <circle cx="12" cy="12" r="3.5" />
      </>
    ),
    plus: <path d="M12 5v14M5 12h14" />,
    back: <path d="m14 5-7 7 7 7M7 12h13" />,
    next: <path d="m9 5 7 7-7 7" />,
    upload: (
      <>
        <path d="M12 16V3m-5 5 5-5 5 5M4 16v5h16v-5" />
      </>
    ),
    close: <path d="m6 6 12 12M18 6 6 18" />,
    trash: (
      <>
        <path d="M4 6h16M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7m4-7v7" />
      </>
    ),
    down: <path d="M12 4v16m-5-5 5 5 5-5" />,
    up: <path d="M12 20V4m-5 5 5-5 5 5" />,
    check: <path d="m5 12 4 4L19 6" />,
    logout: (
      <>
        <path d="M9 4H4v16h5m5-14 6 6-6 6M8 12h12" />
      </>
    ),
    eye: (
      <>
        <path d="M2 12s3-6 10-6 10 6 10 6-3 6-10 6-10-6-10-6Z" />
        <circle cx="12" cy="12" r="2" />
      </>
    ),
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}

export function Loading({ children = "Memuat arsip…", full = false }) {
  return (
    <div className={`loading ${full ? "loading-full" : ""}`} role="status">
      <span className="loading-mark" aria-hidden="true" />
      <span>{children}</span>
    </div>
  );
}

export function ErrorNotice({ error, onRetry }) {
  if (!error) return null;
  return (
    <div className="notice notice-error" role="alert">
      <div>
        <strong>Belum berhasil</strong>
        <p>{typeof error === "string" ? error : error.message}</p>
        {error.requestId && <small>Referensi: {error.requestId}</small>}
      </div>
      {onRetry && (
        <button
          className="button button-small button-outline"
          onClick={onRetry}
        >
          Coba lagi
        </button>
      )}
    </div>
  );
}

export function Dialog({
  title,
  children,
  onClose,
  className = "",
  dismissible = true,
}) {
  const ref = useRef(null);
  useEffect(() => {
    const dialog = ref.current;
    dialog.showModal();
    return () => dialog.close();
  }, []);
  return (
    <dialog
      ref={ref}
      className={`dialog ${className}`}
      aria-labelledby="dialog-title"
      onCancel={(event) => {
        event.preventDefault();
        if (dismissible) onClose();
      }}
    >
      <div className="dialog-heading">
        <h2 id="dialog-title">{title}</h2>
        {dismissible && (
          <button
            type="button"
            className="icon-button"
            aria-label="Tutup dialog"
            onClick={onClose}
          >
            <Icon name="close" />
          </button>
        )}
      </div>
      {children}
    </dialog>
  );
}
