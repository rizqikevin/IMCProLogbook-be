-- +goose Up
ALTER TABLE messages ADD COLUMN edited_at TIMESTAMPTZ;
CREATE OR REPLACE VIEW machine_messages AS
SELECT id, machine_id, text, created_by, created_at, NULL::uuid AS legacy_archive_id, edited_at
FROM messages
UNION ALL
SELECT id, machine_id, ''::text, created_by, created_at, id AS legacy_archive_id, NULL::timestamptz AS edited_at
FROM logbooks;

-- +goose Down
DROP VIEW machine_messages;
CREATE VIEW machine_messages AS
SELECT id, machine_id, text, created_by, created_at, NULL::uuid AS legacy_archive_id
FROM messages
UNION ALL
SELECT id, machine_id, ''::text, created_by, created_at, id AS legacy_archive_id
FROM logbooks;
ALTER TABLE messages DROP COLUMN edited_at;
