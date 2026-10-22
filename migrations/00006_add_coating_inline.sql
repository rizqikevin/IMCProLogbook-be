-- +goose Up
INSERT INTO machines (id, name) VALUES (8, 'COATING INLINE');

-- +goose Down
-- Foreign-key protection prevents removal when this machine has logbook data.
DELETE FROM machines WHERE id = 8 AND name = 'COATING INLINE';
