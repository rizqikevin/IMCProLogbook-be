package repository_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"machine-logbook/internal/domain"
	"machine-logbook/internal/repository"
	"machine-logbook/migrations"
)

func testRepository(t *testing.T) (*repository.Repository, *pgxpool.Pool, domain.User) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	schema := "logbook_test_" + strings.ReplaceAll(newID(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop test schema: %v", err)
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
		database, err := url.Parse(databaseURL)
		if err != nil {
			t.Fatal(err)
		}
		query := database.Query()
		query.Set("search_path", schema)
		database.RawQuery = query.Encode()
		dsn = database.String()
	}
	for range 2 {
		if err := migrations.Up(ctx, dsn); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repo := repository.New(pool)
	user := domain.User{ID: newID(), Username: "operator", Name: "Test operator", Role: "operator", Active: true, PasswordHash: "test-hash"}
	if err := repo.CreateUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	return repo, pool, user
}

func newID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(err)
	}
	data[6] = data[6]&0x0f | 0x40
	data[8] = data[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
}

func newPhoto(userID string) domain.Photo {
	return domain.Photo{ID: newID(), ContentType: "image/jpeg", Size: 1234,
		StorageKey: "photos/" + newID() + ".jpg", SHA256: strings.Repeat("a", 64), UploadedBy: userID}
}

func reserve(t *testing.T, repo *repository.Repository, photos ...domain.Photo) {
	t.Helper()
	keys := make([]string, len(photos))
	for i := range photos {
		keys[i] = photos[i].StorageKey
	}
	if err := repo.ReserveUploads(context.Background(), keys, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func newBook(userID, date string, photos ...domain.Photo) domain.Logbook {
	return domain.Logbook{ID: newID(), Machine: domain.Machine{ID: 1}, Shift: domain.Shift{ID: 1},
		LogDate: date, CreatedBy: userID, Photos: photos}
}

func TestIntegrationReferencesAndPagination(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	machines, err := repo.Machines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(machines))
	for i := range machines {
		names[i] = machines[i].Name
	}
	if !slices.Equal(names, []string{"MAILENDER 222", "MS3", "COATING 1", "COATING 2", "COATING 3"}) {
		t.Fatalf("unexpected seed machines: %v", names)
	}
	shifts, err := repo.Shifts(ctx)
	if err != nil || len(shifts) != 3 || shifts[2].Name != "Shift 3" {
		t.Fatalf("unexpected seed shifts: %v, %v", shifts, err)
	}
	if err := repo.CheckReferences(ctx, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckReferences(ctx, 99, 1); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("missing machine: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET active = FALSE WHERE id = 5`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckReferences(ctx, 5, 1); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("inactive machine: %v", err)
	}
	for _, date := range []string{"2026-09-20", "2026-09-21", "2026-09-22"} {
		photo := newPhoto(user.ID)
		reserve(t, repo, photo)
		book, err := repo.Create(ctx, newBook(user.ID, date, photo))
		if err != nil {
			t.Fatal(err)
		}
		if book.LogDate != date || book.Photos[0].PageNumber != 1 || book.Machine.Name != "MAILENDER 222" {
			t.Fatalf("unexpected created book: %+v", book)
		}
	}
	filter := domain.Filter{MachineID: 1, ShiftID: 1, DateFrom: "2026-09-20", DateTo: "2026-09-22", Limit: 2}
	first, err := repo.List(ctx, filter)
	if err != nil || len(first) != 2 || first[0].LogDate != "2026-09-22" || first[1].LogDate != "2026-09-21" {
		t.Fatalf("first page: %+v, %v", first, err)
	}
	filter.Cursor = &domain.Cursor{Date: first[1].LogDate, ID: first[1].ID}
	second, err := repo.List(ctx, filter)
	if err != nil || len(second) != 1 || second[0].LogDate != "2026-09-20" {
		t.Fatalf("second page: %+v, %v", second, err)
	}
	all, err := repo.List(ctx, domain.Filter{Limit: 20})
	if err != nil || len(all) != 3 {
		t.Fatalf("unfiltered page: %+v, %v", all, err)
	}
}

func TestIntegrationConcurrentCreateAndAppend(t *testing.T) {
	repo, _, user := testRepository(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	start := make(chan struct{})
	type result struct {
		book domain.Logbook
		err  error
	}
	results := make(chan result, 2)
	for range 2 {
		photo := newPhoto(user.ID)
		reserve(t, repo, photo)
		wg.Go(func() {
			<-start
			book, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", photo))
			results <- result{book: book, err: err}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	var book domain.Logbook
	var conflicts int
	for result := range results {
		if result.err == nil {
			book = result.book
		} else if errors.Is(result.err, domain.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(result.err)
		}
	}
	if book.ID == "" || conflicts != 1 {
		t.Fatalf("expected one winner and one conflict, book=%s conflicts=%d", book.ID, conflicts)
	}
	appendResults := make(chan error, 8)
	for range 8 {
		photo := newPhoto(user.ID)
		reserve(t, repo, photo)
		wg.Go(func() {
			_, err := repo.AppendPhotos(ctx, book.ID, []domain.Photo{photo}, 5)
			appendResults <- err
		})
	}
	wg.Wait()
	close(appendResults)
	var appended, limited int
	for err := range appendResults {
		switch {
		case err == nil:
			appended++
		case errors.Is(err, domain.ErrTooManyPhotos):
			limited++
		default:
			t.Fatal(err)
		}
	}
	if appended != 4 || limited != 4 {
		t.Fatalf("append successes=%d limited=%d", appended, limited)
	}
	loaded, err := repo.Get(ctx, book.ID)
	if err != nil || loaded.PhotoCount != 5 || len(loaded.Photos) != 5 {
		t.Fatalf("loaded book: %+v, %v", loaded, err)
	}
	for i, photo := range loaded.Photos {
		if photo.PageNumber != i+1 {
			t.Fatalf("page ordering: %+v", loaded.Photos)
		}
	}
	photo, err := repo.Photo(ctx, book.ID, loaded.Photos[0].ID)
	if err != nil || photo.StorageKey != loaded.Photos[0].StorageKey {
		t.Fatalf("photo lookup: %+v, %v", photo, err)
	}
	if _, err := repo.Photo(ctx, newID(), loaded.Photos[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("photo from another book: %v", err)
	}
}

func TestIntegrationDurableObjectCleanup(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	missing := newPhoto(user.ID)
	if _, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", missing)); !errors.Is(err, domain.ErrUploadExpired) {
		t.Fatalf("unreserved upload: %v", err)
	}
	expired := newPhoto(user.ID)
	if err := repo.ReserveUploads(ctx, []string{expired.StorageKey}, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	keys, err := repo.PendingDeletions(ctx, 20)
	if err != nil || !slices.Equal(keys, []string{expired.StorageKey}) {
		t.Fatalf("claim expired upload: %v, %v", keys, err)
	}
	// Even extending an already claimed reservation must not allow promotion.
	if _, err := pool.Exec(ctx, `UPDATE object_deletions SET not_before = now() + interval '1 hour' WHERE storage_key = $1`, expired.StorageKey); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", expired)); !errors.Is(err, domain.ErrUploadExpired) {
		t.Fatalf("claimed upload promoted: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE object_deletions SET not_before = now() - interval '1 hour' WHERE storage_key = $1`, expired.StorageKey); err != nil {
		t.Fatal(err)
	}
	keys, err = repo.PendingDeletions(ctx, 20)
	if err != nil || !slices.Equal(keys, []string{expired.StorageKey}) {
		t.Fatalf("retry after worker crash: %v, %v", keys, err)
	}
	if err := repo.CompleteDeletion(ctx, expired.StorageKey); err != nil {
		t.Fatal(err)
	}
	photo := newPhoto(user.ID)
	reserve(t, repo, photo)
	book, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", photo))
	if err != nil {
		t.Fatal(err)
	}
	keys, err = repo.PendingDeletions(ctx, 20)
	if err != nil || len(keys) != 0 {
		t.Fatalf("attached photo was queued: %v, %v", keys, err)
	}
	if err := repo.Delete(ctx, book.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, book.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted book exists: %v", err)
	}
	keys, err = repo.PendingDeletions(ctx, 20)
	if err != nil || !slices.Equal(keys, []string{photo.StorageKey}) {
		t.Fatalf("deleted photo was not queued: %v, %v", keys, err)
	}
	if err := repo.CompleteDeletion(ctx, photo.StorageKey); err != nil {
		t.Fatal(err)
	}
	keys, err = repo.PendingDeletions(ctx, 20)
	if err != nil || len(keys) != 0 {
		t.Fatalf("completed deletion remains: %v, %v", keys, err)
	}
}

func TestIntegrationFailedCreatePreservesUploadReservations(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	first, second := newPhoto(user.ID), newPhoto(user.ID)
	second.ID = first.ID
	reserve(t, repo, first, second)
	book := newBook(user.ID, "2026-09-22", first, second)
	if _, err := repo.Create(ctx, book); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected duplicate photo failure, got %v", err)
	}
	if _, err := repo.Get(ctx, book.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("failed transaction left archive metadata: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM object_deletions WHERE NOT claimed`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("rollback lost orphan cleanup reservations: count=%d err=%v", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM logbook_photos`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback retained partial photos: count=%d err=%v", count, err)
	}
}

func TestIntegrationConcurrentAppendAndDelete(t *testing.T) {
	repo, _, user := testRepository(t)
	ctx := context.Background()
	original, appended := newPhoto(user.ID), newPhoto(user.ID)
	reserve(t, repo, original, appended)
	book, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", original))
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	appendResult := make(chan error, 1)
	deleteResult := make(chan error, 1)
	go func() {
		<-start
		_, err := repo.AppendPhotos(ctx, book.ID, []domain.Photo{appended}, 10)
		appendResult <- err
	}()
	go func() {
		<-start
		deleteResult <- repo.Delete(ctx, book.ID)
	}()
	close(start)
	appendErr := <-appendResult
	if appendErr != nil && !errors.Is(appendErr, domain.ErrNotFound) {
		t.Fatal(appendErr)
	}
	if err := <-deleteResult; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, book.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("archive survived delete: %v", err)
	}
	keys, err := repo.PendingDeletions(ctx, 10)
	if err != nil || !slices.Contains(keys, original.StorageKey) {
		t.Fatalf("original photo missing from cleanup: %v, %v", keys, err)
	}
	if appendErr == nil && !slices.Contains(keys, appended.StorageKey) {
		t.Fatalf("committed append was lost by concurrent delete: %v", keys)
	}
}

func sessionFor(user domain.User) domain.Session {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		panic(err)
	}
	return domain.Session{TokenHash: hex.EncodeToString(token[:]), User: user, ExpiresAt: time.Now().Add(time.Hour)}
}

func TestIntegrationSessionRevocation(t *testing.T) {
	repo, _, user := testRepository(t)
	ctx := context.Background()
	loaded, err := repo.UserByUsername(ctx, user.Username)
	if err != nil || loaded.PasswordHash != user.PasswordHash {
		t.Fatalf("user lookup: %+v, %v", loaded, err)
	}
	session := sessionFor(user)
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if actual, err := repo.Session(ctx, session.TokenHash); err != nil || actual.User.ID != user.ID {
		t.Fatalf("session lookup: %+v, %v", actual, err)
	}
	if err := repo.SetPassword(ctx, user.Username, "replacement-hash"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Session(ctx, session.TokenHash); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("reset did not revoke session: %v", err)
	}
	if err := repo.CreateSession(ctx, sessionFor(user)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("stale password created session: %v", err)
	}
	user.PasswordHash = "replacement-hash"
	session = sessionFor(user)
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := repo.DisableUser(ctx, user.Username); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Session(ctx, session.TokenHash); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("disable did not revoke session: %v", err)
	}
	if err := repo.CreateSession(ctx, sessionFor(user)); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("disabled user created session: %v", err)
	}
}

func TestIntegrationConcurrentPasswordReset(t *testing.T) {
	repo, _, user := testRepository(t)
	ctx := context.Background()
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 17)
	sessions := make([]domain.Session, 16)
	for i := range sessions {
		sessions[i] = sessionFor(user)
		wg.Go(func() {
			<-start
			err := repo.CreateSession(ctx, sessions[i])
			if errors.Is(err, domain.ErrUnauthorized) {
				err = nil
			}
			results <- err
		})
	}
	wg.Go(func() {
		<-start
		results <- repo.SetPassword(ctx, user.Username, "new-hash")
	})
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, session := range sessions {
		if _, err := repo.Session(ctx, session.TokenHash); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("old password survived concurrent reset: %v", err)
		}
	}
}

func TestIntegrationExpiredAndLoggedOutSessions(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	expired := sessionFor(user)
	expired.ExpiresAt = time.Now().Add(-time.Hour)
	if err := repo.CreateSession(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Session(ctx, expired.TokenHash); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expired session accepted: %v", err)
	}
	if err := repo.PurgeSessions(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("expired session not purged: count=%d error=%v", count, err)
	}
	session := sessionFor(user)
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSession(ctx, session.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Session(ctx, session.TokenHash); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("logged-out session accepted: %v", err)
	}
}
