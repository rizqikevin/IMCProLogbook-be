-- +goose Up
INSERT INTO machines (id, name) VALUES (7, 'MAILENDER 121');

-- +goose Down
DELETE FROM machines WHERE id = 7 AND name = 'MAILENDER 121';
