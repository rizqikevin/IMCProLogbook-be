package repository

import (
	"context"

	"machine-logbook/internal/domain"
)

func (r *Repository) ReplaceRotatedPhoto(ctx context.Context, user domain.User, machineID int64, parentID, kind string, original, updated domain.Photo, degrees int) (domain.Photo, error) {
	if degrees != 90 && degrees != -90 && degrees != 180 {
		return domain.Photo{}, &domain.ValidationError{Message: "invalid rotation"}
	}
	table, parentColumn, parentTable := "logbook_photos", "logbook_id", "logbooks"
	if kind == "message" {
		table, parentColumn, parentTable = "message_photos", "message_id", "messages"
	} else if kind != "logbook" {
		return domain.Photo{}, domain.ErrNotFound
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Photo{}, err
	}
	defer tx.Rollback(ctx)
	var locked string
	parentQuery := `SELECT id::text FROM ` + parentTable + ` WHERE id=$1::uuid`
	args := []any{parentID}
	if kind == "message" {
		parentQuery += ` AND machine_id=$2 AND EXISTS (SELECT 1 FROM machines WHERE id=$2 AND active)`
		args = append(args, machineID)
	}
	if err := tx.QueryRow(ctx, parentQuery+` FOR UPDATE`, args...).Scan(&locked); err != nil {
		return domain.Photo{}, mapError(err)
	}
	current, err := scanPhoto(tx.QueryRow(ctx, `SELECT `+photoColumns+` FROM `+table+` WHERE id=$1::uuid AND `+parentColumn+`=$2::uuid FOR UPDATE`, original.ID, parentID))
	if err != nil {
		return domain.Photo{}, err
	}
	if user.Role != "admin" && current.UploadedBy != user.ID {
		return domain.Photo{}, domain.ErrForbidden
	}
	if current.SHA256 != original.SHA256 || current.StorageKey != original.StorageKey {
		return domain.Photo{}, domain.ErrConflict
	}
	tag, err := tx.Exec(ctx, `DELETE FROM object_deletions WHERE storage_key=$1 AND NOT claimed AND not_before > clock_timestamp()`, updated.StorageKey)
	if err != nil {
		return domain.Photo{}, err
	}
	if tag.RowsAffected() != 1 {
		return domain.Photo{}, domain.ErrUploadExpired
	}
	result, err := scanPhoto(tx.QueryRow(ctx, `UPDATE `+table+` SET content_type=$2, size=$3, storage_key=$4, sha256=$5 WHERE id=$1::uuid RETURNING `+photoColumns, original.ID, updated.ContentType, updated.Size, updated.StorageKey, updated.SHA256))
	if err != nil {
		return domain.Photo{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE photo_markers SET x=CASE $2::integer WHEN 90 THEN 1-y WHEN -90 THEN y ELSE 1-x END, y=CASE $2::integer WHEN 90 THEN x WHEN -90 THEN 1-x ELSE 1-y END WHERE photo_id=$1::uuid`, original.ID, degrees); err != nil {
		return domain.Photo{}, err
	}
	if kind == "logbook" {
		if _, err := tx.Exec(ctx, `UPDATE logbooks SET updated_at=clock_timestamp() WHERE id=$1::uuid`, parentID); err != nil {
			return domain.Photo{}, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO object_deletions(storage_key, not_before) VALUES ($1, clock_timestamp())`, original.StorageKey); err != nil {
		return domain.Photo{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Photo{}, err
	}
	return result, nil
}
