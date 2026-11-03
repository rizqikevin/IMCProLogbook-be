package domain

import "time"

type PushSubscription struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Endpoint   string    `json:"endpoint"`
	P256DH     string    `json:"p256dh"`
	Auth       string    `json:"auth"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}
