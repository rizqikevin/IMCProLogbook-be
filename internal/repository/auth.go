package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"machine-logbook/internal/domain"
)

func (r *Repository) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	var user domain.User
	err := r.pool.QueryRow(ctx, `SELECT id::text, username, name, role, active, password_hash
		FROM users WHERE username = $1`, username).Scan(&user.ID, &user.Username, &user.Name,
		&user.Role, &user.Active, &user.PasswordHash)
	return user, mapError(err)
}

func (r *Repository) CreateUser(ctx context.Context, user domain.User) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO users(id, username, name, role, active, password_hash)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)`, user.ID, user.Username, user.Name, user.Role, user.Active, user.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserExists
		}
	}
	return mapError(err)
}

func (r *Repository) SetPassword(ctx context.Context, username, passwordHash string) error {
	return r.updateUser(ctx, username, `UPDATE users SET password_hash = $2 WHERE username = $1 RETURNING id::text`, passwordHash)
}

func (r *Repository) DisableUser(ctx context.Context, username string) error {
	return r.updateUser(ctx, username, `UPDATE users SET active = FALSE WHERE username = $1 RETURNING id::text`)
}

func (r *Repository) updateUser(ctx context.Context, username, query string, extra ...any) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	args := append([]any{username}, extra...)
	var id string
	if err := tx.QueryRow(ctx, query, args...).Scan(&id); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1::uuid`, id); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *Repository) CreateSession(ctx context.Context, session domain.Session) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active bool
	var passwordHash string
	// Serialize issuance with password changes and disable operations. The hash
	// must still match the password that was verified before this transaction.
	err = tx.QueryRow(ctx, `SELECT active, password_hash FROM users WHERE id = $1::uuid FOR UPDATE`, session.User.ID).
		Scan(&active, &passwordHash)
	if err == pgx.ErrNoRows {
		return domain.ErrUnauthorized
	}
	if err != nil {
		return err
	}
	if !active || passwordHash != session.User.PasswordHash {
		return domain.ErrUnauthorized
	}
	_, err = tx.Exec(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at)
		VALUES ($1, $2::uuid, $3)`, session.TokenHash, session.User.ID, session.ExpiresAt)
	if err != nil {
		return mapError(err)
	}
	return tx.Commit(ctx)
}

func (r *Repository) Session(ctx context.Context, tokenHash string) (domain.Session, error) {
	var session domain.Session
	err := r.pool.QueryRow(ctx, `SELECT s.token_hash, s.expires_at,
		u.id::text, u.username, u.name, u.role, u.active
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > clock_timestamp() AND u.active`, tokenHash).
		Scan(&session.TokenHash, &session.ExpiresAt, &session.User.ID, &session.User.Username,
			&session.User.Name, &session.User.Role, &session.User.Active)
	if err == pgx.ErrNoRows {
		return domain.Session{}, domain.ErrUnauthorized
	}
	return session, err
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	return err
}

func (r *Repository) PurgeSessions(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= clock_timestamp()`)
	return err
}
