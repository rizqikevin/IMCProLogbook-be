export function Skeleton({
  className = "",
  variant = "text",
  width,
  height,
  style,
  ...props
}) {
  return (
    <span
      className={`skeleton skeleton-${variant} ${className}`.trim()}
      style={{
        width,
        height,
        ...style,
      }}
      aria-hidden="true"
      {...props}
    />
  );
}

export function SkeletonMachineList({ count = 6 }) {
  return (
    <nav className="machine-panel skeleton-machine-panel" aria-label="Memuat mesin…" role="status" aria-busy="true">
      <span className="visually-hidden">Memuat mesin…</span>
      <div className="machine-buttons">
        {Array.from({ length: count }).map((_, index) => (
          <div key={index} className="machine-option machine-option-skeleton" aria-hidden="true">
            <span className="machine-photo-surface">
              <Skeleton variant="rectangular" className="skeleton-machine-photo" />
            </span>
            <span className="machine-card-caption">
              <Skeleton variant="text" width="65%" height="1.1rem" />
              <Skeleton variant="text" width="24px" height="0.85rem" />
            </span>
          </div>
        ))}
      </div>
    </nav>
  );
}

export function SkeletonTimeline({ children = "Memuat pesan…" }) {
  return (
    <div className="skeleton-timeline" role="status" aria-live="polite">
      <div className="skeleton-status-pill">
        <span className="loading-mark" aria-hidden="true" />
        <span>{children}</span>
      </div>
      <div className="skeleton-timeline-bubbles" aria-hidden="true">
        <div className="skeleton-date-badge">
          <Skeleton variant="rounded" width="110px" height="22px" />
        </div>
        <div className="skeleton-bubble skeleton-bubble-other">
          <Skeleton variant="text" width="70px" height="12px" className="skeleton-bubble-meta" />
          <Skeleton variant="text" width="82%" height="15px" />
          <Skeleton variant="text" width="55%" height="15px" />
          <Skeleton variant="text" width="42px" height="11px" className="skeleton-bubble-time" />
        </div>
        <div className="skeleton-bubble skeleton-bubble-own">
          <Skeleton variant="text" width="65px" height="12px" className="skeleton-bubble-meta" />
          <Skeleton variant="rectangular" width="100%" height="120px" className="skeleton-bubble-photo" />
          <Skeleton variant="text" width="90%" height="15px" />
          <Skeleton variant="text" width="42px" height="11px" className="skeleton-bubble-time" />
        </div>
        <div className="skeleton-bubble skeleton-bubble-other">
          <Skeleton variant="text" width="80px" height="12px" className="skeleton-bubble-meta" />
          <Skeleton variant="text" width="70%" height="15px" />
          <Skeleton variant="text" width="42px" height="11px" className="skeleton-bubble-time" />
        </div>
      </div>
    </div>
  );
}

export function SkeletonConversationPage() {
  return (
    <div className="conversation-page skeleton-conversation-page" role="status" aria-label="Memuat mesin…">
      <header className="conversation-heading">
        <Skeleton variant="rounded" width="44px" height="44px" />
        <Skeleton variant="rectangular" width="48px" height="38px" style={{ borderRadius: "4px" }} />
        <div style={{ flex: 1, minWidth: 0 }}>
          <Skeleton variant="text" width="140px" height="1.3rem" />
          <Skeleton variant="text" width="85px" height="0.75rem" style={{ marginTop: "4px" }} />
        </div>
        <div className="conversation-heading-actions">
          <Skeleton variant="rounded" width="84px" height="36px" />
          <Skeleton variant="rounded" width="38px" height="38px" />
        </div>
      </header>
      <section className="conversation-timeline" tabIndex={-1}>
        <SkeletonTimeline>Memuat mesin…</SkeletonTimeline>
      </section>
      <footer className="message-composer">
        <div className="composer-row">
          <Skeleton variant="rounded" width="40px" height="40px" />
          <Skeleton variant="rounded" width="100%" height="44px" />
          <Skeleton variant="rounded" width="40px" height="40px" />
        </div>
      </footer>
    </div>
  );
}

export function SkeletonBook() {
  return (
    <div className="page book-page skeleton-book-page" role="status" aria-label="Memuat buku logbook…">
      <div className="page-heading">
        <div style={{ flex: 1 }}>
          <Skeleton variant="text" width="160px" height="0.85rem" />
          <Skeleton variant="text" width="220px" height="2rem" style={{ margin: "0.4rem 0" }} />
          <Skeleton variant="text" width="110px" height="0.85rem" />
        </div>
        <Skeleton variant="rounded" width="150px" height="46px" />
      </div>
      <div className="shift-nav skeleton-shift-nav">
        <Skeleton variant="rounded" width="110px" height="44px" />
        <Skeleton variant="rounded" width="170px" height="44px" />
        <Skeleton variant="rounded" width="110px" height="44px" />
      </div>
      <div className="book-workspace">
        <section className="page-viewer">
          <div className="viewer-toolbar">
            <Skeleton variant="text" width="110px" height="1rem" />
            <div className="viewer-actions" style={{ display: "flex", gap: "0.5rem" }}>
              <Skeleton variant="rounded" width="65px" height="30px" />
              <Skeleton variant="rounded" width="80px" height="30px" />
            </div>
          </div>
          <SkeletonPhotoStage />
          <div className="viewer-pagination" style={{ display: "flex", justifyContent: "space-between", marginTop: "1rem" }}>
            <Skeleton variant="rounded" width="110px" height="44px" />
            <Skeleton variant="rounded" width="110px" height="44px" />
          </div>
        </section>
      </div>
    </div>
  );
}

export function SkeletonPhotoStage() {
  return (
    <div className="photo-stage skeleton-photo-stage" role="status" aria-label="Memuat halaman…">
      <Skeleton variant="rectangular" width="100%" height="100%" className="skeleton-stage-shimmer" />
      <div className="skeleton-photo-status" aria-hidden="true">
        <span className="loading-mark" />
        <span>Memuat halaman…</span>
      </div>
    </div>
  );
}

export function SkeletonShiftArchive() {
  return (
    <div className="page book-page skeleton-shift-page" role="status" aria-label="Memuat arsip shift…">
      <div className="page-heading">
        <div style={{ flex: 1 }}>
          <Skeleton variant="text" width="150px" height="0.85rem" />
          <Skeleton variant="text" width="200px" height="2rem" style={{ margin: "0.4rem 0" }} />
        </div>
      </div>
      <div className="shift-nav skeleton-shift-nav">
        <Skeleton variant="rounded" width="110px" height="44px" />
        <Skeleton variant="rounded" width="170px" height="44px" />
        <Skeleton variant="rounded" width="110px" height="44px" />
      </div>
      <div
        className="empty-state skeleton-empty-box"
        style={{
          minHeight: "180px",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: "0.75rem",
        }}
      >
        <Skeleton variant="text" width="180px" height="1.5rem" />
        <Skeleton variant="text" width="220px" height="1rem" />
      </div>
    </div>
  );
}

export function SkeletonMessagePhoto({ pageNumber = 1 }) {
  return (
    <div
      className="message-photo-placeholder skeleton-photo-placeholder"
      role="status"
      aria-label={`Memuat foto ${pageNumber}…`}
    >
      <Skeleton variant="rectangular" width="100%" height="100%" className="skeleton-photo-shimmer" />
      <span className="skeleton-photo-status">
        <span className="loading-mark" aria-hidden="true" />
        <span>Memuat foto {pageNumber}…</span>
      </span>
    </div>
  );
}

export function SkeletonPhotoDialog() {
  return (
    <div className="photo-dialog-stage skeleton-photo-dialog" role="status" aria-label="Memuat foto…">
      <Skeleton variant="rectangular" width="100%" height="100%" className="skeleton-stage-shimmer" />
      <span className="skeleton-photo-status">
        <span className="loading-mark" aria-hidden="true" />
        <span>Memuat foto…</span>
      </span>
    </div>
  );
}

export function SkeletonCamera() {
  return (
    <div className="camera-preview-skeleton" role="status" aria-label="Membuka kamera…">
      <Skeleton variant="rectangular" width="100%" height="100%" className="skeleton-camera-shimmer" />
      <span className="skeleton-photo-status" style={{ position: "absolute", zIndex: 2 }}>
        <span className="loading-mark" aria-hidden="true" />
        <span>Membuka kamera…</span>
      </span>
    </div>
  );
}

export function SkeletonAuth() {
  return (
    <div className="loading-full skeleton-auth-wrapper" role="status" aria-label="Memeriksa sesi Anda…">
      <div className="skeleton-auth-card">
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.25rem" }}>
          <Skeleton variant="rounded" width="44px" height="44px" />
          <Skeleton variant="text" width="150px" height="1.4rem" />
        </div>
        <Skeleton variant="text" width="85%" height="1rem" style={{ marginBottom: "1rem" }} />
        <Skeleton variant="rounded" width="100%" height="46px" style={{ marginBottom: "0.85rem" }} />
        <Skeleton variant="rounded" width="100%" height="46px" style={{ marginBottom: "1.25rem" }} />
        <Skeleton variant="rounded" width="100%" height="48px" />
      </div>
      <div className="skeleton-status-pill" style={{ marginTop: "1.5rem" }}>
        <span className="loading-mark" aria-hidden="true" />
        <span>Memeriksa sesi Anda…</span>
      </div>
    </div>
  );
}
