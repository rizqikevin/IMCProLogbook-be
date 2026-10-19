import { Link, Navigate, useParams } from "react-router-dom";
import { useCatalog, useShiftArchive } from "../hooks";
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
  const {
    book,
    loading: shiftLoading,
    error: shiftError,
    retry: shiftRetry,
  } = useShiftArchive(machineId, date, shiftId, {
    enabled: Boolean(machine && valid),
  });
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
  if (shiftLoading) return <Loading />;
  if (book) return <Navigate to={`/logbooks/${book.id}`} replace />;
  return (
    <div className="page book-page">
      <Link className="back-link" to={`/machines/${machineId}`}>
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
      {shiftError ? (
        <ErrorNotice error={shiftError} onRetry={shiftRetry} />
      ) : (
        <section className="empty-state">
          <h2>Belum ada arsip</h2>
          <p>Logbook shift ini belum diunggah.</p>
          <Link
            className="button button-primary"
            to={`/machines/${machineId}`}
          >
            Buka catatan mesin
          </Link>
        </section>
      )}
    </div>
  );
}
