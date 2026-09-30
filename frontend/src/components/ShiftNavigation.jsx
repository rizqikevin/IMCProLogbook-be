import { Link } from "react-router-dom";
import { adjacentShift, shiftPath } from "../lib/shifts";
import { formatDate } from "../lib/format";

export default function ShiftNavigation({ machine, date, shift }) {
  return (
    <nav className="shift-navigation" aria-label="Navigasi shift">
      {[-1, 1].map((direction) => {
        const next = adjacentShift(date, shift, direction);
        return (
          next && (
            <Link
              key={direction}
              className="button button-outline"
              to={shiftPath(machine, next.date, next.shift)}
            >
              <strong>
                {direction === -1 ? "Shift sebelumnya" : "Shift berikutnya"}
              </strong>
              <span>
                {formatDate(next.date)} · Shift {next.shift}
              </span>
            </Link>
          )
        );
      })}
    </nav>
  );
}
