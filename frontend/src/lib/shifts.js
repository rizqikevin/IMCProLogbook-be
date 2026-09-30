export function validDate(value) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value || "")) return false;
  const date = new Date(`${value}T12:00:00Z`);
  return (
    Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value
  );
}

export function adjacentShift(date, shift, direction) {
  if (!validDate(date) || ![1, 2, 3].includes(Number(shift))) return null;
  let next = Number(shift) + direction;
  const day = new Date(`${date}T12:00:00Z`);
  if (next === 4) {
    next = 1;
    day.setUTCDate(day.getUTCDate() + 1);
  }
  if (next === 0) {
    next = 3;
    day.setUTCDate(day.getUTCDate() - 1);
  }
  return { date: day.toISOString().slice(0, 10), shift: next };
}

export function shiftPath(machine, date, shift) {
  return `/machines/${machine}/shifts/${date}/${shift}`;
}
