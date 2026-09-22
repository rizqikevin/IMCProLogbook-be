package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"machine-logbook/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Machines(ctx context.Context) ([]domain.Machine, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM machines WHERE active ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list machines: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Machine, 0)
	for rows.Next() {
		var item domain.Machine
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Shifts(ctx context.Context) ([]domain.Shift, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM shifts WHERE active ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list shifts: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Shift, 0)
	for rows.Next() {
		var item domain.Shift
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) CheckReferences(ctx context.Context, machineID, shiftID int64) error {
	var valid bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM machines WHERE id = $1 AND active)
		AND EXISTS (SELECT 1 FROM shifts WHERE id = $2 AND active)`, machineID, shiftID).Scan(&valid)
	if err != nil {
		return fmt.Errorf("check references: %w", err)
	}
	if !valid {
		return domain.ErrInvalidReference
	}
	return nil
}

func (r *Repository) ReserveUploads(ctx context.Context, keys []string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO object_deletions(storage_key, not_before)
		SELECT unnest($1::text[]), $2::timestamptz`, keys, expiresAt)
	if err != nil {
		return fmt.Errorf("reserve uploads: %w", err)
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, book domain.Logbook) (domain.Logbook, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Logbook{}, err
	}
	defer tx.Rollback(ctx)
	date, err := time.Parse(time.DateOnly, book.LogDate)
	if err != nil {
		return domain.Logbook{}, &domain.ValidationError{Message: "invalid log date"}
	}
	tag, err := tx.Exec(ctx, `INSERT INTO logbooks(id, machine_id, log_date, shift_id, created_by)
		SELECT $1::uuid, m.id, $3::date, s.id, $5::uuid
		FROM machines m CROSS JOIN shifts s
		WHERE m.id = $2 AND m.active AND s.id = $4 AND s.active`,
		book.ID, book.Machine.ID, date, book.Shift.ID, book.CreatedBy)
	if err != nil {
		return domain.Logbook{}, mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Logbook{}, domain.ErrInvalidReference
	}
	if err := insertPhotos(ctx, tx, book.ID, book.Photos, 0); err != nil {
		return domain.Logbook{}, err
	}
	created, err := getBook(ctx, tx, book.ID)
	if err != nil {
		return domain.Logbook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Logbook{}, fmt.Errorf("commit logbook: %w", err)
	}
	return created, nil
}

func (r *Repository) AppendPhotos(ctx context.Context, id string, photos []domain.Photo, maxPhotos int) (domain.Logbook, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Logbook{}, err
	}
	defer tx.Rollback(ctx)
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM logbooks WHERE id = $1::uuid FOR UPDATE`, id).Scan(&lockedID); err != nil {
		return domain.Logbook{}, mapError(err)
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM logbook_photos WHERE logbook_id = $1::uuid`, id).Scan(&count); err != nil {
		return domain.Logbook{}, err
	}
	if count+len(photos) > maxPhotos {
		return domain.Logbook{}, domain.ErrTooManyPhotos
	}
	if err := insertPhotos(ctx, tx, id, photos, count); err != nil {
		return domain.Logbook{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE logbooks SET updated_at = clock_timestamp() WHERE id = $1::uuid`, id); err != nil {
		return domain.Logbook{}, err
	}
	book, err := getBook(ctx, tx, id)
	if err != nil {
		return domain.Logbook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Logbook{}, fmt.Errorf("commit appended photos: %w", err)
	}
	return book, nil
}

func insertPhotos(ctx context.Context, tx pgx.Tx, bookID string, photos []domain.Photo, offset int) error {
	keys := make([]string, len(photos))
	for i, photo := range photos {
		keys[i] = photo.StorageKey
	}
	// Deleting the reservation and adding metadata share one transaction, so a
	// cleanup worker can never claim an object that has been attached to a book.
	tag, err := tx.Exec(ctx, `DELETE FROM object_deletions WHERE storage_key = ANY($1::text[])
		AND NOT claimed AND not_before > clock_timestamp()`, keys)
	if err != nil {
		return fmt.Errorf("promote uploads: %w", err)
	}
	if tag.RowsAffected() != int64(len(photos)) {
		return domain.ErrUploadExpired
	}
	for i, photo := range photos {
		_, err := tx.Exec(ctx, `INSERT INTO logbook_photos
			(id, logbook_id, page_number, content_type, size, storage_key, sha256, uploaded_by)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid)`,
			photo.ID, bookID, offset+i+1, photo.ContentType, photo.Size, photo.StorageKey, photo.SHA256, photo.UploadedBy)
		if err != nil {
			return mapError(err)
		}
	}
	return nil
}

const bookColumns = `b.id::text, b.machine_id, m.name, to_char(b.log_date, 'YYYY-MM-DD'),
	b.shift_id, s.name, b.created_by::text, b.created_at, b.updated_at,
	(SELECT count(*) FROM logbook_photos p WHERE p.logbook_id = b.id)`

func scanBook(row pgx.Row) (domain.Logbook, error) {
	var book domain.Logbook
	err := row.Scan(&book.ID, &book.Machine.ID, &book.Machine.Name, &book.LogDate,
		&book.Shift.ID, &book.Shift.Name, &book.CreatedBy, &book.CreatedAt, &book.UpdatedAt, &book.PhotoCount)
	return book, mapError(err)
}

func getBook(ctx context.Context, tx pgx.Tx, id string) (domain.Logbook, error) {
	book, err := scanBook(tx.QueryRow(ctx, `SELECT `+bookColumns+`
		FROM logbooks b JOIN machines m ON m.id = b.machine_id JOIN shifts s ON s.id = b.shift_id
		WHERE b.id = $1::uuid`, id))
	if err != nil {
		return domain.Logbook{}, err
	}
	rows, err := tx.Query(ctx, `SELECT `+photoColumns+` FROM logbook_photos
		WHERE logbook_id = $1::uuid ORDER BY page_number`, id)
	if err != nil {
		return domain.Logbook{}, err
	}
	defer rows.Close()
	book.Photos = make([]domain.Photo, 0, book.PhotoCount)
	for rows.Next() {
		photo, err := scanPhoto(rows)
		if err != nil {
			return domain.Logbook{}, err
		}
		book.Photos = append(book.Photos, photo)
	}
	if err := rows.Err(); err != nil {
		return domain.Logbook{}, err
	}
	book.PhotoCount = len(book.Photos)
	return book, nil
}

func (r *Repository) Get(ctx context.Context, id string) (domain.Logbook, error) {
	// Keep metadata and pages on the same snapshot during concurrent append/delete.
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return domain.Logbook{}, err
	}
	defer tx.Rollback(ctx)
	book, err := getBook(ctx, tx, id)
	if err != nil {
		return domain.Logbook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Logbook{}, err
	}
	return book, nil
}

func (r *Repository) List(ctx context.Context, filter domain.Filter) ([]domain.Logbook, error) {
	var dateFrom, dateTo, cursorDate any
	var cursorID any
	for _, field := range []struct {
		value  string
		target *any
	}{
		{filter.DateFrom, &dateFrom}, {filter.DateTo, &dateTo},
	} {
		if field.value != "" {
			date, err := time.Parse(time.DateOnly, field.value)
			if err != nil {
				return nil, &domain.ValidationError{Message: "invalid date filter"}
			}
			*field.target = date
		}
	}
	if filter.Cursor != nil {
		date, err := time.Parse(time.DateOnly, filter.Cursor.Date)
		if err != nil {
			return nil, &domain.ValidationError{Message: "invalid cursor date"}
		}
		cursorDate, cursorID = date, filter.Cursor.ID
	}
	rows, err := r.pool.Query(ctx, `SELECT `+bookColumns+`
		FROM logbooks b JOIN machines m ON m.id = b.machine_id JOIN shifts s ON s.id = b.shift_id
		WHERE ($1::bigint = 0 OR b.machine_id = $1)
		  AND ($2::bigint = 0 OR b.shift_id = $2)
		  AND ($3::date IS NULL OR b.log_date >= $3)
		  AND ($4::date IS NULL OR b.log_date <= $4)
		  AND ($5::date IS NULL OR (b.log_date, b.id) < ($5::date, $6::uuid))
		ORDER BY b.log_date DESC, b.id DESC LIMIT $7::integer`,
		filter.MachineID, filter.ShiftID, dateFrom, dateTo, cursorDate, cursorID, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("list logbooks: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Logbook, 0)
	for rows.Next() {
		book, err := scanBook(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, book)
	}
	return items, rows.Err()
}

const photoColumns = `id::text, page_number, content_type, size, storage_key, sha256, uploaded_by::text, created_at`

func scanPhoto(row pgx.Row) (domain.Photo, error) {
	var photo domain.Photo
	err := row.Scan(&photo.ID, &photo.PageNumber, &photo.ContentType, &photo.Size,
		&photo.StorageKey, &photo.SHA256, &photo.UploadedBy, &photo.CreatedAt)
	return photo, mapError(err)
}

func (r *Repository) Photo(ctx context.Context, bookID, photoID string) (domain.Photo, error) {
	return scanPhoto(r.pool.QueryRow(ctx, `SELECT `+photoColumns+` FROM logbook_photos
		WHERE logbook_id = $1::uuid AND id = $2::uuid`, bookID, photoID))
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var lockedID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM logbooks WHERE id = $1::uuid FOR UPDATE`, id).Scan(&lockedID); err != nil {
		return mapError(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO object_deletions(storage_key, not_before)
		SELECT storage_key, clock_timestamp() FROM logbook_photos WHERE logbook_id = $1::uuid`, id); err != nil {
		return fmt.Errorf("queue photo deletion: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM logbooks WHERE id = $1::uuid`, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) PendingDeletions(ctx context.Context, limit int) ([]string, error) {
	// Claims survive process crashes. Object keys are never reused, so retrying
	// an already claimed deletion is safe across multiple worker instances.
	rows, err := r.pool.Query(ctx, `WITH due AS (
		SELECT storage_key FROM object_deletions WHERE not_before <= clock_timestamp()
		ORDER BY not_before, storage_key LIMIT $1::integer FOR UPDATE SKIP LOCKED
	)
	UPDATE object_deletions d SET claimed = TRUE FROM due
	WHERE d.storage_key = due.storage_key RETURNING d.storage_key`, limit)
	if err != nil {
		return nil, fmt.Errorf("claim deletions: %w", err)
	}
	defer rows.Close()
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (r *Repository) CompleteDeletion(ctx context.Context, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM object_deletions WHERE storage_key = $1 AND claimed`, key)
	return err
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return domain.ErrConflict
		case "23503":
			return domain.ErrInvalidReference
		}
	}
	return err
}
