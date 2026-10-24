-- +goose Up
CREATE TABLE photo_markers (
    id uuid PRIMARY KEY,
    photo_id uuid NOT NULL,
    x double precision NOT NULL CHECK (x >= 0 AND x <= 1),
    y double precision NOT NULL CHECK (y >= 0 AND y <= 1),
    created_by uuid NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX photo_markers_photo_id_idx ON photo_markers(photo_id, created_at, id);

-- +goose Down
DROP TABLE photo_markers;
