-- +goose Up
INSERT INTO machines (id, name) VALUES (6, 'RuiYuan');

-- +goose Down
-- Foreign-key protection prevents removal when this machine has archived logbooks.
DELETE FROM machines WHERE id = 6 AND name = 'RuiYuan';
