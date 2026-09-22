package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/storage"
)

type Repository interface {
	Machines(context.Context) ([]domain.Machine, error)
	Shifts(context.Context) ([]domain.Shift, error)
	CheckReferences(context.Context, int64, int64) error
	ReserveUploads(context.Context, []string, time.Time) error
	Create(context.Context, domain.Logbook) (domain.Logbook, error)
	AppendPhotos(context.Context, string, []domain.Photo, int) (domain.Logbook, error)
	Get(context.Context, string) (domain.Logbook, error)
	List(context.Context, domain.Filter) ([]domain.Logbook, error)
	Photo(context.Context, string, string) (domain.Photo, error)
	Delete(context.Context, string) error
	PendingDeletions(context.Context, int) ([]string, error)
	CompleteDeletion(context.Context, string) error
}

type Limits struct {
	MaxPhotosPerUpload  int
	MaxPhotosPerLogbook int
	MaxPhotoBytes       int64
	MaxImagePixels      int64
}

type Upload struct {
	Open func() (io.ReadCloser, error)
	Size int64
}

type Logbooks struct {
	repo   Repository
	store  storage.Store
	limits Limits
	logger *slog.Logger
}

func NewLogbooks(repo Repository, store storage.Store, limits Limits, logger *slog.Logger) *Logbooks {
	if logger == nil {
		logger = slog.Default()
	}
	return &Logbooks{repo: repo, store: store, limits: limits, logger: logger}
}

func (s *Logbooks) Machines(ctx context.Context) ([]domain.Machine, error) {
	return s.repo.Machines(ctx)
}

func (s *Logbooks) Shifts(ctx context.Context) ([]domain.Shift, error) {
	return s.repo.Shifts(ctx)
}

func (s *Logbooks) Create(ctx context.Context, user domain.User, machineID int64, logDate string, shiftID int64, uploads []Upload) (domain.Logbook, error) {
	if machineID <= 0 || shiftID <= 0 {
		return domain.Logbook{}, invalid("machine_id and shift_id must be positive integers")
	}
	if !validDate(logDate) {
		return domain.Logbook{}, invalid("log_date must be a valid date in YYYY-MM-DD format")
	}
	if err := s.checkUploadCount(uploads); err != nil {
		return domain.Logbook{}, err
	}
	if len(uploads) > s.limits.MaxPhotosPerLogbook {
		return domain.Logbook{}, domain.ErrTooManyPhotos
	}
	if err := s.repo.CheckReferences(ctx, machineID, shiftID); err != nil {
		return domain.Logbook{}, err
	}
	now := time.Now().UTC()
	book := domain.Logbook{ID: uuid.NewString(), Machine: domain.Machine{ID: machineID}, LogDate: logDate, Shift: domain.Shift{ID: shiftID}, CreatedBy: user.ID, CreatedAt: now, UpdatedAt: now}
	photos, err := s.preparePhotos(ctx, book.ID, user.ID, uploads)
	if err != nil {
		return domain.Logbook{}, err
	}
	if err := s.storePhotos(ctx, photos, uploads); err != nil {
		return domain.Logbook{}, err
	}
	book.Photos, book.PhotoCount = photos, len(photos)
	created, err := s.repo.Create(ctx, book)
	if err != nil {
		return domain.Logbook{}, fmt.Errorf("create logbook: %w", err)
	}
	return withPhotoURLs(created), nil
}

func (s *Logbooks) Append(ctx context.Context, user domain.User, id string, uploads []Upload) (domain.Logbook, error) {
	if !validUUID(id) {
		return domain.Logbook{}, invalid("invalid logbook ID")
	}
	if err := s.checkUploadCount(uploads); err != nil {
		return domain.Logbook{}, err
	}
	book, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Logbook{}, err
	}
	if len(uploads) > s.limits.MaxPhotosPerLogbook-book.PhotoCount {
		return domain.Logbook{}, domain.ErrTooManyPhotos
	}
	photos, err := s.preparePhotos(ctx, id, user.ID, uploads)
	if err != nil {
		return domain.Logbook{}, err
	}
	if err := s.storePhotos(ctx, photos, uploads); err != nil {
		return domain.Logbook{}, err
	}
	updated, err := s.repo.AppendPhotos(ctx, id, photos, s.limits.MaxPhotosPerLogbook)
	if err != nil {
		return domain.Logbook{}, fmt.Errorf("append logbook photos: %w", err)
	}
	return withPhotoURLs(updated), nil
}

func (s *Logbooks) Get(ctx context.Context, id string) (domain.Logbook, error) {
	if !validUUID(id) {
		return domain.Logbook{}, invalid("invalid logbook ID")
	}
	book, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Logbook{}, err
	}
	return withPhotoURLs(book), nil
}

func (s *Logbooks) List(ctx context.Context, filter domain.Filter) (domain.Page, error) {
	if filter.MachineID < 0 || filter.ShiftID < 0 {
		return domain.Page{}, invalid("machine_id and shift_id must be positive integers")
	}
	if (filter.DateFrom != "" && !validDate(filter.DateFrom)) || (filter.DateTo != "" && !validDate(filter.DateTo)) {
		return domain.Page{}, invalid("date_from and date_to must use YYYY-MM-DD format")
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateFrom > filter.DateTo {
		return domain.Page{}, invalid("date_from must not be after date_to")
	}
	if filter.Cursor != nil && (!validDate(filter.Cursor.Date) || !validUUID(filter.Cursor.ID)) {
		return domain.Page{}, invalid("invalid cursor")
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return domain.Page{}, invalid("limit must be between 1 and 100")
	}
	limit := filter.Limit
	filter.Limit++
	books, err := s.repo.List(ctx, filter)
	if err != nil {
		return domain.Page{}, err
	}
	page := domain.Page{Items: make([]domain.Logbook, 0, min(len(books), limit))}
	if len(books) > limit {
		last := books[limit-1]
		encoded, err := json.Marshal(domain.Cursor{Date: last.LogDate, ID: last.ID})
		if err != nil {
			return domain.Page{}, fmt.Errorf("encode pagination cursor: %w", err)
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
		books = books[:limit]
	}
	for _, book := range books {
		page.Items = append(page.Items, withPhotoURLs(book))
	}
	return page, nil
}

func ParseCursor(raw string) (*domain.Cursor, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 512 {
		return nil, invalid("invalid cursor")
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return nil, invalid("invalid cursor")
	}
	var cursor domain.Cursor
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cursor) != nil || !validDate(cursor.Date) || !validUUID(cursor.ID) {
		return nil, invalid("invalid cursor")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, invalid("invalid cursor")
	}
	return &cursor, nil
}

func (s *Logbooks) OpenPhoto(ctx context.Context, bookID, photoID string) (domain.Photo, io.ReadCloser, error) {
	if !validUUID(bookID) || !validUUID(photoID) {
		return domain.Photo{}, nil, invalid("invalid logbook or photo ID")
	}
	photo, err := s.repo.Photo(ctx, bookID, photoID)
	if err != nil {
		return domain.Photo{}, nil, err
	}
	reader, err := s.store.Open(ctx, photo.StorageKey)
	if err != nil {
		return domain.Photo{}, nil, fmt.Errorf("open photo: %w", err)
	}
	photo.URL = photoURL(bookID, photo.ID)
	return photo, reader, nil
}

func (s *Logbooks) Delete(ctx context.Context, id string) error {
	if !validUUID(id) {
		return invalid("invalid logbook ID")
	}
	return s.repo.Delete(ctx, id)
}

func (s *Logbooks) Cleanup(ctx context.Context) error {
	keys, err := s.repo.PendingDeletions(ctx, 100)
	if err != nil {
		return fmt.Errorf("find pending photo deletions: %w", err)
	}
	var failures []error
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := s.store.Delete(ctx, key); err != nil {
			s.logger.ErrorContext(ctx, "photo cleanup failed", "storage_key", key, "error", err)
			failures = append(failures, fmt.Errorf("delete stored photo: %w", err))
			continue
		}
		if err := s.repo.CompleteDeletion(ctx, key); err != nil {
			failures = append(failures, fmt.Errorf("complete photo deletion: %w", err))
		}
	}
	return errors.Join(failures...)
}

func (s *Logbooks) checkUploadCount(uploads []Upload) error {
	if len(uploads) == 0 {
		return invalid("at least one photo is required")
	}
	if len(uploads) > s.limits.MaxPhotosPerUpload {
		return invalid(fmt.Sprintf("at most %d photos may be uploaded at once", s.limits.MaxPhotosPerUpload))
	}
	return nil
}

func (s *Logbooks) preparePhotos(ctx context.Context, bookID, userID string, uploads []Upload) ([]domain.Photo, error) {
	photos := make([]domain.Photo, 0, len(uploads))
	for index, upload := range uploads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		photo, extension, err := s.validatePhoto(upload)
		if err != nil {
			return nil, fmt.Errorf("photo %d: %w", index+1, err)
		}
		photo.ID = uuid.NewString()
		photo.StorageKey = "logbooks/" + bookID + "/" + photo.ID + extension
		photo.PageNumber = index + 1
		photo.UploadedBy = userID
		photo.CreatedAt = time.Now().UTC()
		photos = append(photos, photo)
	}
	return photos, nil
}

func (s *Logbooks) validatePhoto(upload Upload) (domain.Photo, string, error) {
	if upload.Open == nil || upload.Size <= 0 || upload.Size > s.limits.MaxPhotoBytes {
		return domain.Photo{}, "", invalid(fmt.Sprintf("photo must contain 1 to %d bytes", s.limits.MaxPhotoBytes))
	}
	reader, err := upload.Open()
	if err != nil {
		return domain.Photo{}, "", fmt.Errorf("read upload: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, s.limits.MaxPhotoBytes+1))
	closeErr := reader.Close()
	if readErr != nil {
		return domain.Photo{}, "", fmt.Errorf("read upload: %w", readErr)
	}
	if closeErr != nil {
		return domain.Photo{}, "", fmt.Errorf("close upload: %w", closeErr)
	}
	if int64(len(data)) != upload.Size || int64(len(data)) > s.limits.MaxPhotoBytes {
		return domain.Photo{}, "", invalid("photo size does not match upload size or exceeds its limit")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return domain.Photo{}, "", invalid("photo must be a valid JPEG, PNG, or WebP image")
	}
	var contentType, extension string
	switch format {
	case "jpeg":
		contentType, extension = "image/jpeg", ".jpg"
	case "png":
		contentType, extension = "image/png", ".png"
	case "webp":
		contentType, extension = "image/webp", ".webp"
	default:
		return domain.Photo{}, "", invalid("only JPEG, PNG, and WebP photos are supported")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > s.limits.MaxImagePixels/int64(config.Height) {
		return domain.Photo{}, "", invalid(fmt.Sprintf("photo must not exceed %d pixels", s.limits.MaxImagePixels))
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return domain.Photo{}, "", invalid("photo is corrupt or incomplete")
	}
	hash := sha256.Sum256(data)
	return domain.Photo{ContentType: contentType, Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}, extension, nil
}

func (s *Logbooks) storePhotos(ctx context.Context, photos []domain.Photo, uploads []Upload) error {
	keys := make([]string, len(photos))
	for index, photo := range photos {
		keys[index] = photo.StorageKey
	}
	// Reservations survive uncertain database commits; cleanup only claims expired,
	// unpublished keys, so a successful commit cannot lose its photos on a retry.
	if err := s.repo.ReserveUploads(ctx, keys, time.Now().UTC().Add(24*time.Hour)); err != nil {
		return fmt.Errorf("reserve photo uploads: %w", err)
	}
	for index, photo := range photos {
		reader, err := uploads[index].Open()
		if err != nil {
			return fmt.Errorf("reopen upload: %w", err)
		}
		putErr := s.store.Put(ctx, photo.StorageKey, reader, photo.Size, photo.ContentType)
		closeErr := reader.Close()
		if putErr != nil {
			return fmt.Errorf("store photo %d: %w", index+1, putErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close uploaded photo: %w", closeErr)
		}
	}
	return nil
}

func validDate(value string) bool {
	if len(value) != 10 {
		return false
	}
	parsed, err := time.Parse("2006-01-02", value)
	return err == nil && parsed.Year() >= 1 && parsed.Format("2006-01-02") == value
}

func validUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func withPhotoURLs(book domain.Logbook) domain.Logbook {
	for index := range book.Photos {
		book.Photos[index].URL = photoURL(book.ID, book.Photos[index].ID)
	}
	return book
}

func photoURL(bookID, photoID string) string {
	return "/api/v1/logbooks/" + bookID + "/photos/" + photoID
}
