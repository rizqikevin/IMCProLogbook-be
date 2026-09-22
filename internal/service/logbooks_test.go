package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/storage"
)

const testBookID = "61ff9f5a-abde-445d-8fde-b4375134bc35"
const testUserID = "257c357f-3d01-4ecf-9675-2ba1cc01d5d7"

type logbookRepoStub struct {
	Repository
	checkReferences func(context.Context, int64, int64) error
	reserve         func(context.Context, []string, time.Time) error
	create          func(context.Context, domain.Logbook) (domain.Logbook, error)
	get             func(context.Context, string) (domain.Logbook, error)
	appendPhotos    func(context.Context, string, []domain.Photo, int) (domain.Logbook, error)
	list            func(context.Context, domain.Filter) ([]domain.Logbook, error)
	pending         func(context.Context, int) ([]string, error)
	complete        func(context.Context, string) error
}

func (r *logbookRepoStub) CheckReferences(ctx context.Context, machineID, shiftID int64) error {
	if r.checkReferences != nil {
		return r.checkReferences(ctx, machineID, shiftID)
	}
	return nil
}
func (r *logbookRepoStub) ReserveUploads(ctx context.Context, keys []string, expires time.Time) error {
	return r.reserve(ctx, keys, expires)
}
func (r *logbookRepoStub) Create(ctx context.Context, book domain.Logbook) (domain.Logbook, error) {
	return r.create(ctx, book)
}
func (r *logbookRepoStub) Get(ctx context.Context, id string) (domain.Logbook, error) {
	return r.get(ctx, id)
}
func (r *logbookRepoStub) AppendPhotos(ctx context.Context, id string, photos []domain.Photo, max int) (domain.Logbook, error) {
	return r.appendPhotos(ctx, id, photos, max)
}
func (r *logbookRepoStub) List(ctx context.Context, filter domain.Filter) ([]domain.Logbook, error) {
	return r.list(ctx, filter)
}
func (r *logbookRepoStub) PendingDeletions(ctx context.Context, limit int) ([]string, error) {
	return r.pending(ctx, limit)
}
func (r *logbookRepoStub) CompleteDeletion(ctx context.Context, key string) error {
	return r.complete(ctx, key)
}

type storeStub struct {
	storage.Store
	put    func(context.Context, string, io.Reader, int64, string) error
	delete func(context.Context, string) error
}

func (s *storeStub) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return s.put(ctx, key, body, size, contentType)
}
func (s *storeStub) Delete(ctx context.Context, key string) error {
	return s.delete(ctx, key)
}

func testLogbooks(repo Repository, store storage.Store) *Logbooks {
	return NewLogbooks(repo, store, Limits{MaxPhotosPerUpload: 3, MaxPhotosPerLogbook: 5, MaxPhotoBytes: 1 << 20, MaxImagePixels: 100}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func imageBytes(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.White)
	var output bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&output, img)
	case "jpeg":
		err = jpeg.Encode(&output, img, nil)
	case "gif":
		err = gif.Encode(&output, img, nil)
	default:
		t.Fatal("unsupported test format")
	}
	if err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func uploadBytes(data []byte) Upload {
	return Upload{Size: int64(len(data)), Open: func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }}
}

func TestCreateValidatesEntireBatchBeforeWriting(t *testing.T) {
	service := testLogbooks(&logbookRepoStub{}, &storeStub{})
	_, err := service.Create(context.Background(), domain.User{ID: testUserID}, 1, "2026-09-22", 1, []Upload{uploadBytes(imageBytes(t, "png", 2, 2)), uploadBytes([]byte("not an image"))})
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || !strings.Contains(err.Error(), "photo 2") {
		t.Fatalf("expected invalid second photo, got %v", err)
	}
}

func TestCreateReservesBeforeStorageAndPublishesCompleteBatch(t *testing.T) {
	data := imageBytes(t, "png", 2, 2)
	digest := sha256.Sum256(data)
	var events, reserved []string
	repo := &logbookRepoStub{
		reserve: func(_ context.Context, keys []string, expires time.Time) error {
			events = append(events, "reserve")
			reserved = keys
			if delta := time.Until(expires); delta < 23*time.Hour || delta > 25*time.Hour {
				t.Fatalf("reservation expiry %v", expires)
			}
			return nil
		},
		create: func(_ context.Context, book domain.Logbook) (domain.Logbook, error) {
			events = append(events, "create")
			if book.PhotoCount != 2 || len(book.Photos) != 2 || book.CreatedBy != testUserID {
				t.Fatalf("unexpected book: %+v", book)
			}
			for index, photo := range book.Photos {
				if photo.PageNumber != index+1 || photo.StorageKey != reserved[index] || photo.UploadedBy != testUserID || photo.SHA256 != hex.EncodeToString(digest[:]) {
					t.Fatalf("unexpected photo metadata: %+v", photo)
				}
			}
			return book, nil
		},
	}
	store := &storeStub{put: func(_ context.Context, key string, body io.Reader, size int64, contentType string) error {
		events = append(events, "put")
		actual, err := io.ReadAll(body)
		if err != nil || !bytes.Equal(actual, data) || size != int64(len(data)) || contentType != "image/png" {
			t.Fatalf("bad stored bytes: size=%d type=%s err=%v", size, contentType, err)
		}
		return nil
	}}
	book, err := testLogbooks(repo, store).Create(context.Background(), domain.User{ID: testUserID}, 1, "2026-09-22", 1, []Upload{uploadBytes(data), uploadBytes(data)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"reserve", "put", "put", "create"}) {
		t.Fatalf("unexpected operations: %v", events)
	}
	if book.Photos[0].URL != "/api/v1/logbooks/"+book.ID+"/photos/"+book.Photos[0].ID {
		t.Fatalf("missing authenticated photo URL: %q", book.Photos[0].URL)
	}
}

func TestFailedUploadLeavesReservationsAndNeverPublishesPartialBatch(t *testing.T) {
	for _, failure := range []string{"reserve", "second put", "ambiguous commit"} {
		t.Run(failure, func(t *testing.T) {
			failureErr := errors.New(failure)
			puts, creates := 0, 0
			repo := &logbookRepoStub{
				reserve: func(context.Context, []string, time.Time) error {
					if failure == "reserve" {
						return failureErr
					}
					return nil
				},
				create: func(context.Context, domain.Logbook) (domain.Logbook, error) {
					creates++
					return domain.Logbook{}, failureErr
				},
			}
			store := &storeStub{put: func(context.Context, string, io.Reader, int64, string) error {
				puts++
				if failure == "second put" && puts == 2 {
					return failureErr
				}
				return nil
			}}
			data := imageBytes(t, "png", 2, 2)
			_, err := testLogbooks(repo, store).Create(context.Background(), domain.User{ID: testUserID}, 1, "2026-09-22", 1, []Upload{uploadBytes(data), uploadBytes(data)})
			if !errors.Is(err, failureErr) {
				t.Fatalf("expected failure, got %v", err)
			}
			if failure == "reserve" && puts != 0 {
				t.Fatal("stored photo without durable reservation")
			}
			if failure != "ambiguous commit" && creates != 0 {
				t.Fatal("published partial upload")
			}
			// Delete and CompleteDeletion have no stub: calling either would panic.
		})
	}
}

func TestPhotoValidation(t *testing.T) {
	validPNG := imageBytes(t, "png", 2, 2)
	validJPEG := imageBytes(t, "jpeg", 2, 2)
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		data  []byte
		valid bool
	}{
		{"png", validPNG, true},
		{"jpeg", validJPEG, true},
		{"webp", webp, true},
		{"gif rejected", imageBytes(t, "gif", 2, 2), false},
		{"truncated png", validPNG[:len(validPNG)-8], false},
		{"truncated jpeg", validJPEG[:len(validJPEG)-8], false},
		{"pixel limit", imageBytes(t, "png", 11, 10), false},
		{"empty", nil, false},
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := testLogbooks(nil, nil).validatePhoto(uploadBytes(tc.data))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v got err=%v", tc.valid, err)
			}
		})
	}
	service := testLogbooks(nil, nil)
	service.limits.MaxPhotoBytes = 20
	lyingUpload := uploadBytes(validPNG)
	lyingUpload.Size = 10
	if _, _, err := service.validatePhoto(lyingUpload); err == nil {
		t.Fatal("accepted data exceeding declared size and byte limit")
	}
}

func TestCreateRejectsMetadataWithoutReadingUploads(t *testing.T) {
	for _, date := range []string{"2026-02-29", "2026-9-22", "2026-09-22T00:00:00Z", "0000-01-01"} {
		_, err := testLogbooks(nil, nil).Create(context.Background(), domain.User{}, 1, date, 1, []Upload{{Size: 1}})
		var validation *domain.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("accepted date %q: %v", date, err)
		}
	}
}

func TestAppendRejectsExceededLimitBeforeStorage(t *testing.T) {
	repo := &logbookRepoStub{get: func(context.Context, string) (domain.Logbook, error) {
		return domain.Logbook{ID: testBookID, PhotoCount: 5}, nil
	}}
	_, err := testLogbooks(repo, nil).Append(context.Background(), domain.User{}, testBookID, []Upload{{Size: 1}})
	if !errors.Is(err, domain.ErrTooManyPhotos) {
		t.Fatalf("expected photo limit, got %v", err)
	}
}

func TestListPagination(t *testing.T) {
	books := []domain.Logbook{
		{ID: "d73c8e0a-c756-436c-a618-1619e7d2ec74", LogDate: "2026-09-22"},
		{ID: testBookID, LogDate: "2026-09-21"},
		{ID: "086ee2b2-90f5-4fc8-9eee-f7f13b1a6e5d", LogDate: "2026-09-20"},
	}
	repo := &logbookRepoStub{list: func(_ context.Context, filter domain.Filter) ([]domain.Logbook, error) {
		if filter.Limit != 3 {
			t.Fatalf("expected limit+1, got %d", filter.Limit)
		}
		return books, nil
	}}
	page, err := testLogbooks(repo, nil).List(context.Background(), domain.Filter{Limit: 2})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("page %+v, err %v", page, err)
	}
	cursor, err := ParseCursor(page.NextCursor)
	if err != nil || cursor.ID != books[1].ID || cursor.Date != books[1].LogDate {
		t.Fatalf("cursor %+v, err %v", cursor, err)
	}
	books = books[:2]
	lastPage, err := testLogbooks(repo, nil).List(context.Background(), domain.Filter{Limit: 2})
	if err != nil || lastPage.NextCursor != "" {
		t.Fatalf("unexpected cursor on final page: %+v, err %v", lastPage, err)
	}
}

func TestParseCursorRejectsMalformedValues(t *testing.T) {
	values := []string{"not base64!", strings.Repeat("a", 513)}
	for _, json := range []string{
		`{"date":"2026-02-29","id":"` + testBookID + `"}`,
		`{"date":"2026-09-22","id":"invalid"}`,
		`{"date":"2026-09-22","id":"` + testBookID + `","extra":true}`,
		`{"date":"2026-09-22","id":"` + testBookID + `"} {}`,
	} {
		values = append(values, base64.RawURLEncoding.EncodeToString([]byte(json)))
	}
	for _, value := range values {
		if _, err := ParseCursor(value); err == nil {
			t.Fatalf("accepted malformed cursor %q", value)
		}
	}
}

func TestCleanupRetainsFailedDeletions(t *testing.T) {
	failure := errors.New("storage unavailable")
	var completed []string
	repo := &logbookRepoStub{
		pending:  func(context.Context, int) ([]string, error) { return []string{"failed", "deleted"}, nil },
		complete: func(_ context.Context, key string) error { completed = append(completed, key); return nil },
	}
	store := &storeStub{delete: func(_ context.Context, key string) error {
		if key == "failed" {
			return failure
		}
		return nil
	}}
	err := testLogbooks(repo, store).Cleanup(context.Background())
	if !errors.Is(err, failure) || !reflect.DeepEqual(completed, []string{"deleted"}) {
		t.Fatalf("completed=%v err=%v", completed, err)
	}
}
