-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    username TEXT NOT NULL UNIQUE CHECK (username = lower(username) AND username <> ''),
    name TEXT NOT NULL CHECK (name <> ''),
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('operator', 'admin')),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY CHECK (token_hash ~ '^[0-9a-f]{64}$'),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE machines (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO machines (id, name) VALUES
    (1, 'MAILENDER 222'), (2, 'MS3'), (3, 'COATING 1'),
    (4, 'COATING 2'), (5, 'COATING 3');

CREATE TABLE shifts (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    active BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO shifts (id, name) VALUES (1, 'Shift 1'), (2, 'Shift 2'), (3, 'Shift 3');

CREATE TABLE logbooks (
    id UUID PRIMARY KEY,
    machine_id BIGINT NOT NULL REFERENCES machines(id),
    log_date DATE NOT NULL,
    shift_id BIGINT NOT NULL REFERENCES shifts(id),
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT logbooks_machine_date_shift_key UNIQUE (machine_id, log_date, shift_id)
);
CREATE INDEX logbooks_date_id_idx ON logbooks(log_date DESC, id DESC);
CREATE INDEX logbooks_machine_date_id_idx ON logbooks(machine_id, log_date DESC, id DESC);
CREATE INDEX logbooks_shift_date_id_idx ON logbooks(shift_id, log_date DESC, id DESC);
CREATE INDEX logbooks_created_by_idx ON logbooks(created_by);

CREATE TABLE logbook_photos (
    id UUID PRIMARY KEY,
    logbook_id UUID NOT NULL REFERENCES logbooks(id) ON DELETE CASCADE,
    page_number INTEGER NOT NULL CHECK (page_number > 0),
    content_type TEXT NOT NULL,
    size BIGINT NOT NULL CHECK (size > 0),
    storage_key TEXT NOT NULL UNIQUE CHECK (storage_key <> ''),
    sha256 TEXT NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    uploaded_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (logbook_id, page_number)
);
CREATE INDEX logbook_photos_uploaded_by_idx ON logbook_photos(uploaded_by);

CREATE TABLE object_deletions (
    storage_key TEXT PRIMARY KEY CHECK (storage_key <> ''),
    not_before TIMESTAMPTZ NOT NULL,
    claimed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX object_deletions_not_before_idx ON object_deletions(not_before, storage_key);

-- +goose Down
DROP TABLE object_deletions;
DROP TABLE logbook_photos;
DROP TABLE logbooks;
DROP TABLE shifts;
DROP TABLE machines;
DROP TABLE sessions;
DROP TABLE users;
