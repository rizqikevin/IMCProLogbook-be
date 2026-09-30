import { useEffect, useRef, useState } from "react";
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom";
import { useCatalog } from "../hooks";
import { request, queryString } from "../lib/api";
import { formatDate, yesterday } from "../lib/format";
import { ErrorNotice, Icon, Loading } from "../components/common";
import MachinePhoto from "../components/MachinePhoto";

export default function Archives() {
  const catalog = useCatalog();
  const [params, setParams] = useSearchParams();
  const navigate = useNavigate();
  const creating = params.get("action") === "new";
  const machine = params.get("machine_id") || "";
  const shift = params.get("shift_id") || "";
  const dateFrom = params.get("date_from") || "";
  const dateTo = params.get("date_to") || "";
  const query = queryString({
    machine_id: machine,
    shift_id: shift,
    date_from: dateFrom,
    date_to: dateTo,
    limit: 20,
  });
  const [state, setState] = useState({
    items: [],
    cursor: "",
    loading: true,
    error: null,
  });
  const [moreError, setMoreError] = useState(null);
  const [moreLoading, setMoreLoading] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const moreController = useRef(null);
  const generation = useRef(0);
  const rangeError =
    dateFrom && dateTo && dateFrom > dateTo
      ? "Tanggal akhir harus sama atau setelah tanggal awal."
      : "";
  useEffect(() => {
    const controller = new AbortController();
    moreController.current?.abort();
    generation.current += 1;
    setMoreLoading(false);
    setMoreError(null);
    if (!machine || rangeError) {
      setState({ items: [], cursor: "", loading: false, error: null });
      return;
    }
    setState({ items: [], cursor: "", loading: true, error: null });
    request(`/logbooks?${query}`, { signal: controller.signal })
      .then((page) => {
        setState({
          items: page.items,
          cursor: page.next_cursor || "",
          loading: false,
          error: null,
        });
      })
      .catch((error) => {
        if (error.name !== "AbortError")
          setState({ items: [], cursor: "", loading: false, error });
      });
    return () => {
      controller.abort();
      moreController.current?.abort();
    };
  }, [query, attempt, rangeError, machine]);

  function filter(key, value) {
    setParams(
      (old) => {
        const next = new URLSearchParams(old);
        if (value) next.set(key, value);
        else next.delete(key);
        return next;
      },
      { replace: true },
    );
  }
  async function loadMore() {
    if (moreLoading) return;
    const current = generation.current;
    const controller = new AbortController();
    moreController.current = controller;
    setMoreLoading(true);
    setMoreError(null);
    try {
      const page = await request(
        `/logbooks?${query}&cursor=${encodeURIComponent(state.cursor)}`,
        { signal: controller.signal },
      );
      if (current === generation.current)
        setState((old) => ({
          ...old,
          items: [
            ...old.items,
            ...page.items.filter(
              (item) => !old.items.some((existing) => existing.id === item.id),
            ),
          ],
          cursor: page.next_cursor || "",
        }));
    } catch (error) {
      if (error.name !== "AbortError" && current === generation.current)
        setMoreError(error);
    } finally {
      if (current === generation.current) setMoreLoading(false);
    }
  }
  const selectedMachine = catalog.machines.find(
    (item) => String(item.id) === machine,
  );
  if (machine && !catalog.loading && !catalog.error && !selectedMachine)
    return <Navigate to="/" replace />;
  const hasFilters = Boolean(shift || dateFrom || dateTo);
  return (
    <div className="page archives-page">
      <div className="page-heading">
        <div>
          <p className="section-label">Catatan produksi</p>
          <h1>
            {machine ? selectedMachine?.name || "Arsip mesin" : "Pilih mesin"}
          </h1>
          <p className="muted">
            {machine
              ? "Arsip logbook berdasarkan tanggal dan shift."
              : creating
                ? `Pilih mesin untuk input berikutnya. Tanggal awal ${formatDate(yesterday())} (H-1)${shift ? ` · Shift ${shift}` : ""}. Bisa diubah sebelum mengambil foto.`
                : "Pilih mesin untuk membuka atau menambahkan arsip logbook."}
          </p>
        </div>
        {machine ? (
          <Link
            to={`/new?machine_id=${machine}`}
            className="button button-primary"
          >
            <Icon name="plus" />
            Arsip baru
          </Link>
        ) : null}
      </div>
      <ErrorNotice error={catalog.error} onRetry={catalog.retry} />
      {catalog.loading && <Loading />}
      <div className="archive-workspace">
        {!machine && (
          <aside className="machine-panel" aria-label="Pilih mesin">
            <div className="machine-gallery-heading">
              <div>
                <h2>Mesin produksi</h2>
                <p>
                  {creating
                    ? "Pilih mesin untuk mulai mengambil foto."
                    : "Pilih mesin untuk melihat arsipnya."}
                </p>
              </div>
            </div>
            <div className="machine-buttons">
              {catalog.machines.map((item, index) => (
                <button
                  key={item.id}
                  className="machine-option"
                  onClick={() =>
                    creating
                      ? navigate(
                          `/new?${queryString({ machine_id: item.id, shift_id: shift })}`,
                        )
                      : setParams({ machine_id: String(item.id) })
                  }
                >
                  <span className="machine-photo-surface">
                    <MachinePhoto name={item.name} decorative />
                  </span>
                  <span className="machine-card-caption">
                    <span>{item.name}</span>
                    <span className="machine-number" aria-hidden="true">
                      {String(index + 1).padStart(2, "0")}
                    </span>
                  </span>
                </button>
              ))}
            </div>
          </aside>
        )}
        {machine && (
          <section className="archive-content" aria-label="Daftar arsip">
            <Link className="back-link" to="/">
              Pilih mesin lain
            </Link>
            <div className="archive-machine-context">
              <MachinePhoto name={selectedMachine?.name || ""} decorative />
              <div>
                <strong>{selectedMachine?.name}</strong>
                <span>Gunakan tanggal dan shift untuk menemukan catatan.</span>
              </div>
            </div>
            <div className="filter-bar">
              <label>
                Dari tanggal
                <input
                  aria-label="Dari tanggal"
                  type="date"
                  value={dateFrom}
                  onChange={(event) => filter("date_from", event.target.value)}
                />
              </label>
              <span className="date-divider" aria-hidden="true">
                /
              </span>
              <label>
                Sampai tanggal
                <input
                  aria-label="Sampai tanggal"
                  type="date"
                  value={dateTo}
                  min={dateFrom || undefined}
                  onChange={(event) => filter("date_to", event.target.value)}
                />
              </label>
              <label className="shift-filter">
                Shift
                <select
                  value={shift}
                  onChange={(event) => filter("shift_id", event.target.value)}
                >
                  <option value="">Semua shift</option>
                  {catalog.shifts.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              {hasFilters && (
                <button
                  className="text-button reset-filter"
                  onClick={() => setParams({ machine_id: machine })}
                >
                  Reset filter
                </button>
              )}
            </div>
            {rangeError && <ErrorNotice error={rangeError} />}
            <div className="list-heading">
              <h2>Arsip tersimpan</h2>
              {!state.loading && !state.error && (
                <span>{state.items.length} arsip ditampilkan</span>
              )}
            </div>
            {state.loading ? (
              <Loading />
            ) : state.error ? (
              <ErrorNotice
                error={state.error}
                onRetry={() => setAttempt((value) => value + 1)}
              />
            ) : !rangeError && state.items.length === 0 ? (
              <div className="empty-state">
                <span className="empty-number" aria-hidden="true">
                  /
                </span>
                <h3>
                  {hasFilters
                    ? "Belum ada arsip yang cocok."
                    : "Halaman pertama dimulai di sini."}
                </h3>
                <p>
                  {hasFilters
                    ? "Coba mesin, rentang tanggal, atau shift yang lain."
                    : "Foto logbook mesin dan simpan seluruh halamannya dalam satu arsip."}
                </p>
                {hasFilters ? (
                  <button
                    className="button button-outline"
                    onClick={() => setParams({ machine_id: machine })}
                  >
                    Hapus filter
                  </button>
                ) : (
                  <Link
                    className="button button-primary"
                    to={`/new?machine_id=${machine}`}
                  >
                    <Icon name="camera" />
                    Buat arsip pertama
                  </Link>
                )}
              </div>
            ) : (
              <>
                <div className="archive-table-head" aria-hidden="true">
                  <span>Tanggal logbook</span>
                  <span>Mesin</span>
                  <span>Shift</span>
                  <span>Halaman</span>
                  <span />
                </div>
                <ol className="archive-list">
                  {state.items.map((book) => (
                    <li key={book.id}>
                      <Link
                        to={`/logbooks/${book.id}`}
                        className="archive-row"
                        aria-label={`Buka arsip ${book.machine.name}, ${formatDate(book.log_date)}, ${book.shift.name}`}
                      >
                        <time dateTime={book.log_date} className="archive-date">
                          <strong>{book.log_date.slice(8)}</strong>
                          <span>
                            {formatDate(book.log_date, {
                              day: undefined,
                              month: "short",
                            })}
                          </span>
                        </time>
                        <strong className="archive-machine">
                          {book.machine.name}
                        </strong>
                        <span className="archive-shift">{book.shift.name}</span>
                        <span className="archive-count">
                          {book.photo_count}
                          <span> halaman</span>
                        </span>
                        <span className="row-next">
                          <Icon name="next" />
                        </span>
                      </Link>
                    </li>
                  ))}
                </ol>
                <ErrorNotice error={moreError} onRetry={loadMore} />
                {state.cursor && (
                  <div className="load-more">
                    <button
                      className="button button-outline"
                      onClick={loadMore}
                      disabled={moreLoading}
                    >
                      {moreLoading
                        ? "Memuat arsip berikutnya…"
                        : "Muat arsip berikutnya"}
                    </button>
                  </div>
                )}
              </>
            )}
          </section>
        )}
      </div>
      <div className="archive-note">
        <span>Arsip berdasarkan tanggal logbook</span>
        <p>Tanggal mengikuti catatan shift, bukan waktu foto diunggah.</p>
      </div>
    </div>
  );
}
