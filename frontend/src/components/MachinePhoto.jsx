const photos = {
  "MAILENDER 222": "mailender",
  "MAILENDER 121": "mailender",
  MS3: "ms3",
  "COATING 1": "coating",
  "COATING 2": "coating",
  "COATING 3": "coating",
  RUIYUAN: "ruiyuan",
};

export default function MachinePhoto({ name, decorative = false }) {
  const photo = photos[name?.trim().toUpperCase()];
  if (!photo) return null;
  return (
    <img
      className="machine-photo"
      src={`/machine/${photo}.png`}
      alt={decorative ? "" : `Mesin ${name}`}
      loading="lazy"
      decoding="async"
      width="640"
      height="400"
    />
  );
}
