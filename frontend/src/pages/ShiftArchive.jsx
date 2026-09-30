import { useEffect, useState } from "react";
import { Link, Navigate, useParams } from "react-router-dom";
import { useCatalog } from "../hooks";
import { request, queryString } from "../lib/api";
import { validDate } from "../lib/shifts";
import { formatDate } from "../lib/format";
import { ErrorNotice, Loading } from "../components/common";
import ShiftNavigation from "../components/ShiftNavigation";

export default function ShiftArchive() {
  const { machineId, date, shiftId } = useParams();
  const catalog = useCatalog();
  const machine = catalog.machines.find(
    (item) => String(item.id) === machineId,
  );
  const valid = validDate(date) && ["1", "2", "3"].includes(shiftId);
  const [state, setState] = useState({ loading: true });
  const [attempt, setAttempt] = useState(0);
  const key = `${machineId}/${date}/${shiftId}`;
  useEffect(() => {
    if (!machine || !valid) return;
    const controller = new AbortController();
    setState({ loading: true, key });
    request(
      `/logbooks?${queryString({ machine_id: machineId, date_from: date, date_to: date, shift_id: shiftId, limit: 1 })}`,
      { signal: controller.signal },
    )
      .then((result) =>
        setState({ key, book: result.items[0], loading: false }),
      )
      .catch((error) => {
        if (error.name !== "AbortError")
          setState({ key, error, loading: false });
      });
    return () => controller.abort();
  }, [machineId, date, shiftId, Boolean(machine), valid, attempt]);
  if (catalog.loading) return <Loading />;
  if (catalog.error)
    return (
      <div className="page">
        <ErrorNotice error={catalog.error} onRetry={catalog.retry} />
      </div>
    );
  if (!valid || !machine)
    return (
      <div className="page">
        <h1>Arsip tidak ditemukan</h1>
        <Link className="button button-outline" to="/">
          Pilih mesin
        </Link>
      </div>
    );
  if (state.key !== key || state.loading) return <Loading />;
  if (state.book) return <Navigate to={`/logbooks/${state.book.id}`} replace />;
  return (
    <div className="page book-page">
      <Link className="back-link" to={`/?machine_id=${machineId}`}>
        Arsip {machine.name}
      </Link>
      <div className="page-heading">
        <div>
          <p className="section-label">
            {formatDate(date)} · Shift {shiftId}
          </p>
          <h1>{machine.name}</h1>
        </div>
      </div>
      <ShiftNavigation machine={machineId} date={date} shift={shiftId} />
      {state.error ? (
        <ErrorNotice
          error={state.error}
          onRetry={() => setAttempt((value) => value + 1)}
        />
      ) : (
        <section className="empty-state">
          <h2>Belum ada arsip</h2>
          <p>Logbook shift ini belum diunggah.</p>
          <Link
            className="button button-primary"
            to={`/new?${queryString({ machine_id: machineId, date, shift_id: shiftId })}`}
          >
            Tambah logbook
          </Link>
        </section>
      )}
    </div>
  );
}
