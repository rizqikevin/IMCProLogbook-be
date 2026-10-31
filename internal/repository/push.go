package repository

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"machine-logbook/internal/domain"
	"time"
)

func (r *Repository) UpsertPushSubscription(ctx context.Context, sub domain.PushSubscription) error {
	if sub.ID == "" {
		sub.ID = uuid.NewString()
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO push_subscriptions(id,user_id,endpoint,p256dh,auth,last_seen_at,revoked_at)
      VALUES($1,$2,$3,$4,$5,clock_timestamp(),NULL)
      ON CONFLICT(user_id,endpoint) DO UPDATE SET p256dh=EXCLUDED.p256dh, auth=EXCLUDED.auth, last_seen_at=clock_timestamp(), revoked_at=NULL`, sub.ID, sub.UserID, sub.Endpoint, sub.P256DH, sub.Auth)
	if err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}
	return nil
}

func (r *Repository) PushSubscriptions(ctx context.Context, userID string) ([]domain.PushSubscription, error) {
	rows, err := r.pool.Query(ctx, `SELECT id,user_id,endpoint,p256dh,auth,created_at,last_seen_at FROM push_subscriptions WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	defer rows.Close()
	var out []domain.PushSubscription
	for rows.Next() {
		var s domain.PushSubscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256DH, &s.Auth, &s.CreatedAt, &s.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *Repository) RevokePushSubscription(ctx context.Context, userID, endpoint string) error {
	_, err := r.pool.Exec(ctx, `UPDATE push_subscriptions SET revoked_at=clock_timestamp() WHERE user_id=$1 AND endpoint=$2`, userID, endpoint)
	return err
}

func (r *Repository) DeletePushSubscription(ctx context.Context, endpoint string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint=$1`, endpoint)
	return err
}

var _ = time.Time{}
