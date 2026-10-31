-- +goose Up
CREATE TABLE push_subscriptions (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint text NOT NULL,
    p256dh text NOT NULL,
    auth text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    last_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
    revoked_at timestamptz
);
CREATE UNIQUE INDEX push_subscriptions_user_endpoint_idx ON push_subscriptions(user_id, endpoint);
CREATE INDEX push_subscriptions_active_user_idx ON push_subscriptions(user_id) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE push_subscriptions;
