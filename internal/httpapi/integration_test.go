package httpapi_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"machine-logbook/internal/domain"
	"machine-logbook/internal/httpapi"
	"machine-logbook/internal/repository"
	"machine-logbook/internal/service"
	"machine-logbook/internal/storage"
	"machine-logbook/migrations"
)

type integrationApp struct {
	handler http.Handler
	books   *service.Logbooks
	pool    *pgxpool.Pool
	objects string
}

func newIntegrationApp(t *testing.T) integrationApp {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run HTTP integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := "logbook_http_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop HTTP test schema: %v", err)
		}
		admin.Close()
	})
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	dsn := databaseURL + " search_path=" + schema
	if strings.Contains(databaseURL, "://") {
		u, err := url.Parse(databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		query := u.Query()
		query.Set("search_path", schema)
		u.RawQuery = query.Encode()
		dsn = u.String()
	}
	if err := migrations.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	objects := t.TempDir()
	store, err := storage.NewLocal(objects)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	repo := repository.New(pool)
	auth := service.NewAuth(repo, time.Hour)
	for _, role := range []string{"operator", "admin"} {
		if err := auth.Provision(ctx, role, "Test "+role, integrationPassword, role); err != nil {
			t.Fatal(err)
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	books := service.NewLogbooks(repo, store, service.Limits{
		MaxPhotosPerUpload: 5, MaxPhotosPerLogbook: 10, MaxPhotoBytes: 4 << 20, MaxImagePixels: 4_000_000,
	}, logger)
	handler := httpapi.New(books, auth, httpapi.Options{
		Logger: logger, MaxRequestBytes: 8 << 20, MaxConcurrentUploads: 2,
		RequestTimeout: 30 * time.Second, Ready: pool.Ping,
	})
	return integrationApp{handler: handler, books: books, pool: pool, objects: objects}
}

const integrationPassword = "test-password-only-123"

func (app integrationApp) request(method, path, token, contentType string, body io.Reader) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	app.handler.ServeHTTP(recorder, req)
	return recorder
}

func requireStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d, want=%d: %s", response.Code, status, response.Body.String())
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}

func decodeBody[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func (app integrationApp) login(t *testing.T, username string) service.LoginResult {
	t.Helper()
	body, err := json.Marshal(map[string]string{"username": username, "password": integrationPassword})
	if err != nil {
		t.Fatal(err)
	}
	response := app.request(http.MethodPost, "/api/v1/auth/login", "", "application/json", bytes.NewReader(body))
	requireStatus(t, response, http.StatusOK)
	if strings.Contains(response.Body.String(), "password") {
		t.Fatal("login response exposes password data")
	}
	result := decodeBody[service.LoginResult](t, response)
	if result.AccessToken == "" || result.TokenType != "Bearer" || !result.ExpiresAt.After(time.Now()) {
		t.Fatalf("invalid login result: %+v", result)
	}
	return result
}

func uploadBody(t *testing.T, create bool, files ...[]byte) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if create {
		for _, field := range [][2]string{{"machine_id", "1"}, {"log_date", "2026-09-22"}, {"shift_id", "2"}} {
			if err := writer.WriteField(field[0], field[1]); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i, data := range files {
		part, err := writer.CreateFormFile("photos", "page-"+strconv.Itoa(i+1)+".bin")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func integrationImages(t *testing.T) ([]byte, []byte) {
	t.Helper()
	large := image.NewNRGBA(image.Rect(0, 0, 640, 640))
	for y := range 640 {
		for x := range 640 {
			large.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 100, A: 255})
		}
	}
	var pngBody, jpegBody bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&pngBody, large); err != nil {
		t.Fatal(err)
	}
	if pngBody.Len() <= 1<<20 {
		t.Fatal("PNG fixture must exceed the multipart in-memory threshold")
	}
	small := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			small.Set(x, y, color.RGBA{R: 230, G: 40, B: 40, A: 255})
		}
	}
	if err := jpeg.Encode(&jpegBody, small, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return pngBody.Bytes(), jpegBody.Bytes()
}

func requireNoMultipartFiles(t *testing.T, directory string) {
	t.Helper()
	files, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("multipart temporary files remain: %v", files)
	}
}

func TestIntegrationArchiveHTTPWorkflow(t *testing.T) {
	app := newIntegrationApp(t)
	spill := t.TempDir()
	t.Setenv("TMPDIR", spill)
	ctx := context.Background()

	requireStatus(t, app.request(http.MethodGet, "/health/live", "", "", nil), http.StatusOK)
	requireStatus(t, app.request(http.MethodGet, "/health/ready", "", "", nil), http.StatusOK)
	unauthorized := app.request(http.MethodGet, "/api/v1/machines", "", "", nil)
	requireStatus(t, unauthorized, http.StatusUnauthorized)
	if unauthorized.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing bearer authentication challenge")
	}
	badLogin := app.request(http.MethodPost, "/api/v1/auth/login", "", "application/json",
		strings.NewReader(`{"username":"operator","password":"incorrect-password"}`))
	requireStatus(t, badLogin, http.StatusUnauthorized)
	operator, admin := app.login(t, "operator"), app.login(t, "admin")
	me := app.request(http.MethodGet, "/api/v1/auth/me", operator.AccessToken, "", nil)
	requireStatus(t, me, http.StatusOK)
	if actual := decodeBody[domain.User](t, me); actual.ID != operator.User.ID || actual.Role != "operator" {
		t.Fatalf("incorrect operator identity: %+v", actual)
	}
	machinesResponse := app.request(http.MethodGet, "/api/v1/machines", operator.AccessToken, "", nil)
	requireStatus(t, machinesResponse, http.StatusOK)
	machines := decodeBody[struct{ Items []domain.Machine }](t, machinesResponse)
	names := make([]string, len(machines.Items))
	for i, machine := range machines.Items {
		names[i] = machine.Name
	}
	if !slices.Equal(names, []string{"MAILENDER 222", "MS3", "COATING 1", "COATING 2", "COATING 3"}) {
		t.Fatalf("unexpected machines: %v", names)
	}
	shiftsResponse := app.request(http.MethodGet, "/api/v1/shifts", operator.AccessToken, "", nil)
	requireStatus(t, shiftsResponse, http.StatusOK)
	shifts := decodeBody[struct{ Items []domain.Shift }](t, shiftsResponse)
	if len(shifts.Items) != 3 || shifts.Items[0].Name != "Shift 1" || shifts.Items[2].Name != "Shift 3" {
		t.Fatalf("unexpected shifts: %+v", shifts.Items)
	}

	pngData, jpegData := integrationImages(t)
	body, contentType := uploadBody(t, true, pngData, jpegData)
	created := app.request(http.MethodPost, "/api/v1/logbooks", operator.AccessToken, contentType, body)
	requireStatus(t, created, http.StatusCreated)
	requireNoMultipartFiles(t, spill)
	book := decodeBody[domain.Logbook](t, created)
	bookURL := "/api/v1/logbooks/" + book.ID
	if created.Header().Get("Location") != bookURL || book.PhotoCount != 2 || len(book.Photos) != 2 || book.CreatedBy != operator.User.ID {
		t.Fatalf("invalid created archive: %+v", book)
	}
	if book.Machine.Name != "MAILENDER 222" || book.Shift.Name != "Shift 2" || book.LogDate != "2026-09-22" {
		t.Fatalf("incorrect archive references: %+v", book)
	}
	for i, data := range [][]byte{pngData, jpegData} {
		photo := book.Photos[i]
		hash := sha256.Sum256(data)
		if photo.PageNumber != i+1 || photo.Size != int64(len(data)) || photo.SHA256 != hex.EncodeToString(hash[:]) || photo.UploadedBy != operator.User.ID {
			t.Fatalf("photo order or metadata changed at page %d: %+v", i+1, photo)
		}
		if photo.URL != bookURL+"/photos/"+photo.ID {
			t.Fatalf("incorrect protected photo URL: %s", photo.URL)
		}
		get := app.request(http.MethodGet, photo.URL, operator.AccessToken, "", nil)
		requireStatus(t, get, http.StatusOK)
		if !bytes.Equal(get.Body.Bytes(), data) || get.Header().Get("Content-Type") != photo.ContentType || get.Header().Get("ETag") != `"`+photo.SHA256+`"` {
			t.Fatalf("photo bytes or content headers differ for page %d", i+1)
		}
		head := app.request(http.MethodHead, photo.URL, operator.AccessToken, "", nil)
		requireStatus(t, head, http.StatusOK)
		if head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(len(data)) {
			t.Fatalf("invalid HEAD photo response: headers=%v body=%d", head.Header(), head.Body.Len())
		}
		requireStatus(t, app.request(http.MethodGet, photo.URL, "", "", nil), http.StatusUnauthorized)
	}
	if book.Photos[0].ContentType != "image/png" || book.Photos[1].ContentType != "image/jpeg" {
		t.Fatal("file MIME must be detected from image bytes, regardless of filename")
	}
	if strings.Contains(created.Body.String(), "storage_key") {
		t.Fatal("API exposes private storage keys")
	}

	body, contentType = uploadBody(t, false, jpegData)
	appendResponse := app.request(http.MethodPost, bookURL+"/photos", operator.AccessToken, contentType, body)
	requireStatus(t, appendResponse, http.StatusOK)
	appended := decodeBody[domain.Logbook](t, appendResponse)
	if appended.PhotoCount != 3 || len(appended.Photos) != 3 || appended.Photos[2].PageNumber != 3 || appended.Photos[0].ID != book.Photos[0].ID {
		t.Fatalf("append changed page ordering: %+v", appended)
	}
	detailResponse := app.request(http.MethodGet, bookURL, operator.AccessToken, "", nil)
	requireStatus(t, detailResponse, http.StatusOK)
	detail := decodeBody[domain.Logbook](t, detailResponse)
	if detail.PhotoCount != 3 || len(detail.Photos) != 3 {
		t.Fatalf("detail does not include complete archive: %+v", detail)
	}
	listResponse := app.request(http.MethodGet, "/api/v1/logbooks?machine_id=1&shift_id=2&date_from=2026-09-22&date_to=2026-09-22&limit=1", operator.AccessToken, "", nil)
	requireStatus(t, listResponse, http.StatusOK)
	page := decodeBody[domain.Page](t, listResponse)
	if len(page.Items) != 1 || page.Items[0].ID != book.ID || page.Items[0].PhotoCount != 3 || page.NextCursor != "" {
		t.Fatalf("incorrect filtered page: %+v", page)
	}
	emptyResponse := app.request(http.MethodGet, "/api/v1/logbooks?machine_id=2", operator.AccessToken, "", nil)
	requireStatus(t, emptyResponse, http.StatusOK)
	if empty := decodeBody[domain.Page](t, emptyResponse); empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty list must return an empty JSON array: %+v", empty)
	}

	body, contentType = uploadBody(t, true, jpegData)
	duplicate := app.request(http.MethodPost, "/api/v1/logbooks", operator.AccessToken, contentType, body)
	requireStatus(t, duplicate, http.StatusConflict)
	requireNoMultipartFiles(t, spill)
	corrupt := make([]byte, len(pngData))
	copy(corrupt, pngData[:32])
	body, contentType = uploadBody(t, true, corrupt)
	malformed := app.request(http.MethodPost, "/api/v1/logbooks", operator.AccessToken, contentType, body)
	requireStatus(t, malformed, http.StatusBadRequest)
	requireNoMultipartFiles(t, spill)

	rows, err := app.pool.Query(ctx, `SELECT storage_key FROM logbook_photos WHERE logbook_id = $1::uuid ORDER BY page_number`, book.ID)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(keys) != 3 {
		t.Fatalf("stored photo keys: %v, %v", keys, err)
	}
	requireStatus(t, app.request(http.MethodDelete, bookURL, operator.AccessToken, "", nil), http.StatusForbidden)
	requireStatus(t, app.request(http.MethodGet, bookURL, operator.AccessToken, "", nil), http.StatusOK)
	requireStatus(t, app.request(http.MethodDelete, bookURL, admin.AccessToken, "", nil), http.StatusNoContent)
	requireStatus(t, app.request(http.MethodGet, bookURL, operator.AccessToken, "", nil), http.StatusNotFound)
	for _, key := range keys {
		if _, err := os.Stat(filepath.Join(app.objects, filepath.FromSlash(key))); err != nil {
			t.Fatalf("object removed before durable cleanup: %v", err)
		}
	}
	if err := app.books.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if _, err := os.Stat(filepath.Join(app.objects, filepath.FromSlash(key))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cleanup left deleted archive photo: %s, %v", key, err)
		}
	}
	// A rejected duplicate has uploaded files but no metadata. Expire its
	// reservation to verify the same worker also collects abandoned uploads.
	if _, err := app.pool.Exec(ctx, `UPDATE object_deletions SET not_before = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if err := app.books.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(app.objects, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			t.Errorf("orphan object remains after cleanup: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, app.request(http.MethodPost, "/api/v1/auth/logout", operator.AccessToken, "", nil), http.StatusNoContent)
	requireStatus(t, app.request(http.MethodGet, "/api/v1/auth/me", operator.AccessToken, "", nil), http.StatusUnauthorized)
	requireNoMultipartFiles(t, spill)
}
