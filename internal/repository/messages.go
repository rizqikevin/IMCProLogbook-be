package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"machine-logbook/internal/domain"
)

func (r *Repository) CheckMachine(ctx context.Context, machineID int64) error {
	var active bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM machines WHERE id = $1 AND active)`, machineID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return domain.ErrInvalidReference
	}
	return nil
}

func (r *Repository) CreateMessage(ctx context.Context, message domain.Message) (domain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Message{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO messages(id, machine_id, text, created_by)
		SELECT $1::uuid, id, $3, $4::uuid FROM machines WHERE id = $2 AND active
		RETURNING created_at`, message.ID, message.MachineID, message.Text, message.Author.ID).Scan(&message.CreatedAt)
	if err == pgx.ErrNoRows {
		return domain.Message{}, domain.ErrInvalidReference
	}
	if err != nil {
		return domain.Message{}, mapError(err)
	}
	if err := tx.QueryRow(ctx, `SELECT name FROM users WHERE id = $1::uuid`, message.Author.ID).Scan(&message.Author.Name); err != nil {
		return domain.Message{}, mapError(err)
	}
	keys := make([]string, len(message.Photos))
	for i, photo := range message.Photos {
		keys[i] = photo.StorageKey
	}
	if len(keys) > 0 {
		// Promotion shares the metadata transaction, just like archive uploads.
		tag, err := tx.Exec(ctx, `DELETE FROM object_deletions WHERE storage_key = ANY($1::text[])
			AND NOT claimed AND not_before > clock_timestamp()`, keys)
		if err != nil {
			return domain.Message{}, err
		}
		if tag.RowsAffected() != int64(len(keys)) {
			return domain.Message{}, domain.ErrUploadExpired
		}
	}
	for i := range message.Photos {
		photo := &message.Photos[i]
		err := tx.QueryRow(ctx, `INSERT INTO message_photos
			(id, message_id, page_number, content_type, size, storage_key, sha256, uploaded_by)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid) RETURNING created_at`,
			photo.ID, message.ID, photo.PageNumber, photo.ContentType, photo.Size, photo.StorageKey, photo.SHA256, photo.UploadedBy).Scan(&photo.CreatedAt)
		if err != nil {
			return domain.Message{}, mapError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func (r *Repository) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var cursorTime, cursorID any
	if filter.Cursor != nil {
		cursorTime, cursorID = filter.Cursor.CreatedAt, filter.Cursor.ID
	}
	rows, err := tx.Query(ctx, `SELECT t.id::text, t.machine_id, t.text, t.created_by::text, u.name, t.created_at,
		COALESCE(b.id::text, ''), COALESCE(to_char(b.log_date, 'YYYY-MM-DD'), ''), COALESCE(s.id, 0), COALESCE(s.name, '')
		FROM machine_messages t JOIN users u ON u.id = t.created_by
		LEFT JOIN logbooks b ON b.id = t.legacy_archive_id LEFT JOIN shifts s ON s.id = b.shift_id
		WHERE t.machine_id = $1 AND ($2::timestamptz IS NULL OR (t.created_at, t.id) < ($2::timestamptz, $3::uuid))
		AND ($5::text = '' OR strpos(lower(t.text), lower($5::text)) > 0)
		ORDER BY t.created_at DESC, t.id DESC LIMIT $4`, filter.MachineID, cursorTime, cursorID, filter.Limit, filter.Query)
	if err != nil {
		return nil, err
	}
	items := make([]domain.Message, 0)
	for rows.Next() {
		var message domain.Message
		var legacy domain.LegacyArchive
		if err := rows.Scan(&message.ID, &message.MachineID, &message.Text, &message.Author.ID, &message.Author.Name,
			&message.CreatedAt, &legacy.ID, &legacy.LogDate, &legacy.Shift.ID, &legacy.Shift.Name); err != nil {
			rows.Close()
			return nil, err
		}
		if legacy.ID != "" {
			message.LegacyArchive = &legacy
		}
		message.Photos = make([]domain.Photo, 0)
		items = append(items, message)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Fetch both photo kinds in one query on the same snapshot as their messages.
	ids := make([]string, len(items))
	positions := make(map[string]int, len(items))
	for i, message := range items {
		ids[i], positions[message.ID] = message.ID, i
	}
	photos, err := tx.Query(ctx, `SELECT message_id::text, `+photoColumns+` FROM message_photos WHERE message_id = ANY($1::uuid[])
		UNION ALL SELECT logbook_id::text, `+photoColumns+` FROM logbook_photos WHERE logbook_id = ANY($1::uuid[])
		ORDER BY 1, 3`, ids)
	if err != nil {
		return nil, err
	}
	for photos.Next() {
		var id string
		var photo domain.Photo
		if err := photos.Scan(&id, &photo.ID, &photo.PageNumber, &photo.ContentType, &photo.Size, &photo.StorageKey,
			&photo.SHA256, &photo.UploadedBy, &photo.CreatedAt); err != nil {
			photos.Close()
			return nil, err
		}
		index := positions[id]
		items[index].Photos = append(items[index].Photos, photo)
	}
	photos.Close()
	if err := photos.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) MessagePhoto(ctx context.Context, machineID int64, messageID, photoID string) (domain.Photo, error) {
	return scanPhoto(r.pool.QueryRow(ctx, `SELECT `+photoColumns+` FROM message_photos
		WHERE message_id = $1::uuid AND id = $2::uuid
		AND EXISTS (SELECT 1 FROM messages m JOIN machines machine ON machine.id = m.machine_id
			WHERE m.id = $1::uuid AND m.machine_id = $3 AND machine.active)`, messageID, photoID, machineID))
}

func (r *Repository) DeleteMessage(ctx context.Context, machineID int64, messageID, userID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var owned bool
	if err := tx.QueryRow(ctx, `SELECT m.created_by = $3::uuid FROM messages m
		JOIN machines machine ON machine.id = m.machine_id
		WHERE m.id = $1::uuid AND m.machine_id = $2 AND machine.active
		FOR UPDATE OF m`, messageID, machineID, userID).Scan(&owned); err != nil {
		return mapError(err)
	}
	if !owned {
		return domain.ErrForbidden
	}
	if _, err := tx.Exec(ctx, `INSERT INTO object_deletions(storage_key, not_before)
		SELECT storage_key, clock_timestamp() FROM message_photos WHERE message_id = $1::uuid`, messageID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM message_photos WHERE message_id = $1::uuid`, messageID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM messages WHERE id = $1::uuid`, messageID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
