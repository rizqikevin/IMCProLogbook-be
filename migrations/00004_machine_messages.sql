-- +goose Up
CREATE TABLE messages (
    id UUID PRIMARY KEY,
    machine_id BIGINT NOT NULL REFERENCES machines(id),
    text TEXT NOT NULL CHECK (char_length(text) <= 20000),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX messages_machine_created_id_idx ON messages(machine_id, created_at DESC, id DESC);
CREATE INDEX logbooks_machine_created_id_idx ON logbooks(machine_id, created_at DESC, id DESC);

CREATE TABLE message_photos (
    id UUID PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES messages(id),
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size > 0),
    storage_key TEXT NOT NULL UNIQUE CHECK (storage_key <> ''),
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    uploaded_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (message_id, page_number)
);

-- Historical rows and objects stay in place; legacy API writes remain visible.
CREATE VIEW machine_messages AS
SELECT id, machine_id, text, created_by, created_at, NULL::uuid AS legacy_archive_id
FROM messages
UNION ALL
SELECT id, machine_id, ''::text, created_by, created_at, id AS legacy_archive_id
FROM logbooks;

-- +goose Down
-- Refuse rollback if it would discard chat content or photo metadata.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM messages) THEN
        RAISE EXCEPTION 'Cannot roll back machine messages while messages exist';
    END IF;
END $$;
-- +goose StatementEnd
DROP VIEW machine_messages;
DROP TABLE message_photos;
DROP TABLE messages;
DROP INDEX logbooks_machine_created_id_idx;
