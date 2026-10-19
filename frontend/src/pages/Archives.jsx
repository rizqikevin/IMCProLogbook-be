import { Link } from "react-router-dom";
import { useMachines } from "../hooks";
import { ErrorNotice, SkeletonMachineList } from "../components/common";
import MachinePhoto from "../components/MachinePhoto";

export default function Archives() {
  const catalog = useMachines();
  return (
    <div className="page archives-page">
      <div className="page-heading">
        <div><h1>Pilih mesin</h1><p className="muted">Baca dan kirim catatan untuk mesin yang dipilih.</p></div>
      </div>
      <ErrorNotice error={catalog.error} onRetry={catalog.retry} />
      {catalog.loading ? <SkeletonMachineList /> : !catalog.error && (
        catalog.machines.length ? <nav className="machine-panel" aria-label="Pilih mesin">
          <div className="machine-buttons">
            {catalog.machines.map((machine, index) => (
              <Link key={machine.id} className="machine-option" to={`/machines/${machine.id}`}>
                <span className="machine-photo-surface"><MachinePhoto name={machine.name} decorative /></span>
                <span className="machine-card-caption"><span>{machine.name}</span><span className="machine-number" aria-hidden="true">{String(index + 1).padStart(2, "0")}</span></span>
              </Link>
            ))}
          </div>
        </nav> : <div className="empty-state"><h2>Belum ada mesin aktif</h2><p>Hubungi administrator untuk menambahkan mesin.</p></div>
      )}
    </div>
  );
}
