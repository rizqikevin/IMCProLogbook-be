package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"machine-logbook/internal/domain"
)

func (s *Logbooks) CreateMessage(ctx context.Context, user domain.User, machineID int64, text string, uploads []Upload) (domain.Message, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if machineID <= 0 {
		return domain.Message{}, invalid("machine_id must be a positive integer")
	}
	if err := validateMessageText(text); err != nil {
		return domain.Message{}, err
	}
	if strings.TrimSpace(text) == "" && len(uploads) == 0 {
		return domain.Message{}, invalid("nonblank text or at least one photo is required")
	}
	if len(uploads) > 0 {
		if err := s.checkUploadCount(uploads); err != nil {
			return domain.Message{}, err
		}
		if len(uploads) > s.limits.MaxPhotosPerLogbook {
			return domain.Message{}, domain.ErrTooManyPhotos
		}
	}
	if err := s.repo.CheckMachine(ctx, machineID); err != nil {
		return domain.Message{}, err
	}
	message := domain.Message{ID: uuid.NewString(), MachineID: machineID, Text: text,
		Author: domain.MessageAuthor{ID: user.ID, Name: user.Name}, CreatedAt: time.Now().UTC()}
	photos, err := s.preparePhotos(ctx, message.ID, user.ID, uploads)
	if err != nil {
		return domain.Message{}, err
	}
	for i := range photos {
		photos[i].StorageKey = strings.Replace(photos[i].StorageKey, "logbooks/", "messages/", 1)
	}
	message.Photos = photos
	if len(photos) > 0 {
		if err := s.storePhotos(ctx, photos, uploads); err != nil {
			return domain.Message{}, err
		}
	}
	created, err := s.repo.CreateMessage(ctx, message)
	if err != nil {
		return domain.Message{}, fmt.Errorf("create message: %w", err)
	}
	return withMessagePhotoURLs(created), nil
}

func validateMessageText(text string) error {
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) || utf8.RuneCountInString(text) > 20000 {
		return invalid("text must contain at most 20000 Unicode characters and no NUL characters")
	}
	return nil
}

func (s *Logbooks) EditMessage(ctx context.Context, user domain.User, machineID int64, messageID, text string) (domain.Message, error) {
	if !validUUID(user.ID) {
		return domain.Message{}, domain.ErrUnauthorized
	}
	if machineID <= 0 || !validUUID(messageID) {
		return domain.Message{}, invalid("invalid machine or message ID")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if err := validateMessageText(text); err != nil {
		return domain.Message{}, err
	}
	message, err := s.repo.EditMessage(ctx, machineID, messageID, user.ID, text)
	if err != nil {
		return domain.Message{}, err
	}
	return withMessagePhotoURLs(message), nil
}

func ParseMessageCursor(raw string) (*domain.MessageCursor, error) {
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
	var cursor domain.MessageCursor
	decoder := json.NewDecoder(bytes.NewReader(decoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&cursor) != nil || cursor.MachineID <= 0 || cursor.CreatedAt.IsZero() || !validUUID(cursor.ID) || decoder.Decode(new(any)) != io.EOF {
		return nil, invalid("invalid cursor")
	}
	return &cursor, nil
}

func (s *Logbooks) ListMessages(ctx context.Context, filter domain.MessageFilter) (domain.MessagePage, error) {
	if filter.MachineID <= 0 {
		return domain.MessagePage{}, invalid("machine_id must be a positive integer")
	}
	if !utf8.ValidString(filter.Query) || strings.ContainsRune(filter.Query, 0) {
		return domain.MessagePage{}, invalid("q must contain valid Unicode characters and no NUL characters")
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if utf8.RuneCountInString(filter.Query) > 200 {
		return domain.MessagePage{}, invalid("q must contain at most 200 Unicode characters")
	}
	queryHash := ""
	if filter.Query != "" {
		digest := sha256.Sum256([]byte(filter.Query))
		queryHash = hex.EncodeToString(digest[:])
	}
	if filter.Cursor != nil && (filter.Cursor.MachineID != filter.MachineID || filter.Cursor.CreatedAt.IsZero() || !validUUID(filter.Cursor.ID) || filter.Cursor.QueryHash != queryHash) {
		return domain.MessagePage{}, invalid("invalid cursor for machine or query")
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		return domain.MessagePage{}, invalid("limit must be between 1 and 100")
	}
	if err := s.repo.CheckMachine(ctx, filter.MachineID); err != nil {
		return domain.MessagePage{}, err
	}
	limit := filter.Limit
	filter.Limit++
	items, err := s.repo.ListMessages(ctx, filter)
	if err != nil {
		return domain.MessagePage{}, err
	}
	page := domain.MessagePage{Items: make([]domain.Message, 0, min(len(items), limit))}
	if len(items) > limit {
		last := items[limit-1]
		encoded, err := json.Marshal(domain.MessageCursor{MachineID: filter.MachineID, CreatedAt: last.CreatedAt, ID: last.ID, QueryHash: queryHash})
		if err != nil {
			return domain.MessagePage{}, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
		items = items[:limit]
	}
	for i := len(items) - 1; i >= 0; i-- {
		page.Items = append(page.Items, withMessagePhotoURLs(items[i]))
	}
	return page, nil
}

func messagePhotoURL(machineID int64, messageID, photoID string) string {
	return "/api/v1/machines/" + strconv.FormatInt(machineID, 10) + "/messages/" + messageID + "/photos/" + photoID
}

func withMessagePhotoURLs(message domain.Message) domain.Message {
	if message.Photos == nil {
		message.Photos = make([]domain.Photo, 0)
	}
	for i := range message.Photos {
		if message.LegacyArchive != nil {
			message.Photos[i].URL = photoURL(message.LegacyArchive.ID, message.Photos[i].ID)
		} else {
			message.Photos[i].URL = messagePhotoURL(message.MachineID, message.ID, message.Photos[i].ID)
		}
	}
	return message
}

func (s *Logbooks) OpenMessagePhoto(ctx context.Context, machineID int64, messageID, photoID string) (domain.Photo, io.ReadCloser, error) {
	if machineID <= 0 || !validUUID(messageID) || !validUUID(photoID) {
		return domain.Photo{}, nil, invalid("invalid machine, message or photo ID")
	}
	photo, err := s.repo.MessagePhoto(ctx, machineID, messageID, photoID)
	if err != nil {
		return domain.Photo{}, nil, err
	}
	reader, err := s.store.Open(ctx, photo.StorageKey)
	if err != nil {
		return domain.Photo{}, nil, fmt.Errorf("open message photo: %w", err)
	}
	photo.URL = messagePhotoURL(machineID, messageID, photoID)
	return photo, reader, nil
}

func (s *Logbooks) DeleteMessage(ctx context.Context, user domain.User, machineID int64, messageID string) error {
	if !validUUID(user.ID) {
		return domain.ErrUnauthorized
	}
	if machineID <= 0 || !validUUID(messageID) {
		return invalid("invalid machine or message ID")
	}
	return s.repo.DeleteMessage(ctx, machineID, messageID, user.ID)
}
func (s *Logbooks) ValidateMarkerMessagePhoto(ctx context.Context, machineID int64, messageID, photoID string) error {
	if machineID <= 0 || !validUUID(messageID) || !validUUID(photoID) {
		return invalid("invalid photo reference")
	}
	_, err := s.repo.MessagePhoto(ctx, machineID, messageID, photoID)
	return err
}
func (s *Logbooks) ListPhotoMarkers(ctx context.Context, photoID, parentID, parentKind string) ([]domain.PhotoMarker, error) {
	if !validUUID(photoID) || !validUUID(parentID) || (parentKind != "message" && parentKind != "logbook") {
		return nil, invalid("invalid photo reference")
	}
	return s.repo.ListPhotoMarkers(ctx, photoID, parentID, parentKind)
}
func (s *Logbooks) AddPhotoMarker(ctx context.Context, user domain.User, photoID, parentID, parentKind string, x, y float64) (domain.PhotoMarker, error) {
	if !validUUID(user.ID) {
		return domain.PhotoMarker{}, domain.ErrUnauthorized
	}
	if !validUUID(photoID) || !validUUID(parentID) || (parentKind != "message" && parentKind != "logbook") || math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) || x < 0 || x > 1 || y < 0 || y > 1 {
		return domain.PhotoMarker{}, invalid("invalid photo reference or coordinates")
	}
	uploader, err := s.repo.PhotoMarkerAccess(ctx, photoID, parentID, parentKind)
	if err != nil {
		return domain.PhotoMarker{}, err
	}
	if user.Role != "admin" && user.ID != uploader {
		return domain.PhotoMarker{}, domain.ErrForbidden
	}
	return s.repo.AddPhotoMarker(ctx, photoID, parentID, parentKind, domain.PhotoMarker{ID: uuid.NewString(), X: x, Y: y, CreatedBy: user.ID}, user.Role)
}
func (s *Logbooks) DeletePhotoMarker(ctx context.Context, user domain.User, photoID, markerID, parentID, parentKind string) error {
	if !validUUID(user.ID) {
		return domain.ErrUnauthorized
	}
	if !validUUID(photoID) || !validUUID(markerID) || !validUUID(parentID) || (parentKind != "message" && parentKind != "logbook") {
		return invalid("invalid photo reference")
	}
	canEdit, err := s.PhotoMarkerCanEdit(ctx, user, photoID, parentID, parentKind)
	if err != nil {
		return err
	}
	if !canEdit {
		return domain.ErrForbidden
	}
	return s.repo.DeletePhotoMarker(ctx, photoID, markerID, parentID, parentKind, user.ID, user.Role)
}
func (s *Logbooks) PhotoMarkerCanEdit(ctx context.Context, user domain.User, photoID, parentID, parentKind string) (bool, error) {
	if !validUUID(user.ID) || !validUUID(photoID) || !validUUID(parentID) || (parentKind != "message" && parentKind != "logbook") {
		return false, invalid("invalid photo reference")
	}
	uploader, err := s.repo.PhotoMarkerAccess(ctx, photoID, parentID, parentKind)
	if err != nil {
		return false, err
	}
	return user.Role == "admin" || user.ID == uploader, nil
}
