package repository_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"machine-logbook/internal/domain"
)

func TestIntegrationEditMessagePreservesHistoryAndRejectsEmptyText(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	message, err := repo.CreateMessage(ctx, domain.Message{ID: newID(), MachineID: 1, Text: "original", Author: domain.MessageAuthor{ID: user.ID}})
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `UPDATE messages SET created_at = $1 WHERE id = $2`, stamp, message.ID); err != nil {
		t.Fatal(err)
	}
	unchanged, err := repo.EditMessage(ctx, 1, message.ID, user.ID, "original")
	if err != nil || unchanged.EditedAt != nil {
		t.Fatalf("initial no-op: %+v %v", unchanged, err)
	}
	for _, text := range []string{"", " \t\n", "\u2003"} {
		_, err := repo.EditMessage(ctx, 1, message.ID, user.ID, text)
		var validation *domain.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("blank text %q accepted: %v", text, err)
		}
	}
	if _, err := repo.EditMessage(ctx, 1, message.ID, newID(), "intrusion"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-owner edit: %v", err)
	}
	items, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(items) != 1 || items[0].Text != "original" || items[0].EditedAt != nil {
		t.Fatalf("rejected edits persisted: %+v %v", items, err)
	}
	edited, err := repo.EditMessage(ctx, 1, message.ID, user.ID, "updated")
	if err != nil || edited.EditedAt == nil || !edited.EditedAt.After(stamp) || !edited.CreatedAt.Equal(stamp) || edited.Photos == nil || edited.Author.Name != user.Name {
		t.Fatalf("old message edit: %+v %v", edited, err)
	}
	unchanged, err = repo.EditMessage(ctx, 1, message.ID, user.ID, "updated")
	if err != nil || unchanged.EditedAt == nil || !unchanged.EditedAt.Equal(*edited.EditedAt) {
		t.Fatalf("no-op changed timestamp: %+v %v", unchanged, err)
	}
	items, err = repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(items) != 1 || items[0].Text != "updated" || items[0].EditedAt == nil || !items[0].EditedAt.Equal(*edited.EditedAt) || !items[0].CreatedAt.Equal(stamp) {
		t.Fatalf("edit not persisted: %+v %v", items, err)
	}
}

func TestIntegrationMessagesPreserveArchivesAndPageByStableKey(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	photo := newPhoto(user.ID)
	reserve(t, repo, photo)
	book, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", photo))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(legacy) != 1 {
		t.Fatalf("legacy=%+v err=%v", legacy, err)
	}
	item := legacy[0]
	if item.ID != book.ID || item.Text != "" || item.Author.ID != user.ID || item.Author.Name != user.Name || !item.CreatedAt.Equal(book.CreatedAt) || item.LegacyArchive == nil || item.LegacyArchive.LogDate != book.LogDate || item.LegacyArchive.Shift != book.Shift || len(item.Photos) != 1 || item.Photos[0].StorageKey != photo.StorageKey || item.Photos[0].ID != photo.ID {
		t.Fatalf("archive changed: %+v", item)
	}
	if _, err := repo.Get(ctx, book.ID); err != nil {
		t.Fatalf("old detail broken: %v", err)
	}
	stamp := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `UPDATE logbooks SET created_at = $1 WHERE id = $2`, stamp.Add(-time.Hour), book.ID); err != nil {
		t.Fatal(err)
	}
	ids := []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"}
	for _, id := range ids {
		message, err := repo.CreateMessage(ctx, domain.Message{ID: id, MachineID: 1, Text: "note", Author: domain.MessageAuthor{ID: user.ID, Name: "untrusted name"}, Photos: []domain.Photo{}})
		if err != nil || message.Author.Name != user.Name || message.CreatedAt.IsZero() {
			t.Fatalf("created=%+v err=%v", message, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE messages SET created_at = $1 WHERE id = $2`, stamp, id); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 2})
	if err != nil || len(first) != 2 || first[0].ID != ids[2] || first[1].ID != ids[1] {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	older, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 2, Cursor: &domain.MessageCursor{MachineID: 1, CreatedAt: stamp, ID: ids[1]}})
	if err != nil || len(older) != 2 || older[0].ID != ids[0] || older[1].ID != book.ID {
		t.Fatalf("older=%+v err=%v", older, err)
	}
	other, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 2, Limit: 20})
	if err != nil || len(other) != 0 {
		t.Fatalf("machine isolation=%+v err=%v", other, err)
	}
}

func TestIntegrationMessageSearchFiltersBeforeLimit(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	photo := newPhoto(user.ID)
	reserve(t, repo, photo)
	book, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", photo))
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC)
	var matches []domain.Message
	for i := range 27 {
		text := "unrelated newer message"
		if i < 3 {
			text = `Motor 100%_\ READY 界`
		}
		message := domain.Message{ID: newID(), MachineID: 1, Text: text, Author: domain.MessageAuthor{ID: user.ID}}
		if i == 0 {
			attachment := newPhoto(user.ID)
			attachment.PageNumber = 1
			reserve(t, repo, attachment)
			message.Photos = []domain.Photo{attachment}
		}
		created, err := repo.CreateMessage(ctx, message)
		if err != nil {
			t.Fatal(err)
		}
		created.CreatedAt = stamp.Add(time.Duration(i) * time.Minute)
		if _, err := pool.Exec(ctx, `UPDATE messages SET created_at = $1 WHERE id = $2`, created.CreatedAt, created.ID); err != nil {
			t.Fatal(err)
		}
		if i < 3 {
			matches = append(matches, created)
		}
	}
	if _, err := repo.CreateMessage(ctx, domain.Message{ID: newID(), MachineID: 2, Text: matches[0].Text, Author: domain.MessageAuthor{ID: user.ID}}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"motor", "READY", "%", "_", `\`, `%_\`, "界"} {
		page, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Query: query, Limit: 2})
		if err != nil || len(page) != 2 || page[0].ID != matches[2].ID || page[1].ID != matches[1].ID {
			t.Fatalf("query=%q page=%+v err=%v", query, page, err)
		}
		older, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Query: query, Limit: 2, Cursor: &domain.MessageCursor{MachineID: 1, CreatedAt: page[1].CreatedAt, ID: page[1].ID}})
		if err != nil || len(older) != 1 || older[0].ID != matches[0].ID || len(older[0].Photos) != 1 {
			t.Fatalf("query=%q older=%+v err=%v", query, older, err)
		}
	}
	all, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 100})
	if err != nil || len(all) != 28 {
		t.Fatalf("cleared search=%+v err=%v", all, err)
	}
	archiveFound := false
	for _, message := range all {
		if message.ID == book.ID {
			archiveFound = message.LegacyArchive != nil && len(message.Photos) == 1
		}
	}
	if !archiveFound {
		t.Fatal("clearing search did not restore archive")
	}
	missing, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Query: "no match", Limit: 20})
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
}

func TestIntegrationMessagePhotosPromoteAtomicallyAndCheckMembership(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	photo := newPhoto(user.ID)
	photo.PageNumber = 1
	message := domain.Message{ID: newID(), MachineID: 1, Author: domain.MessageAuthor{ID: user.ID}, Photos: []domain.Photo{photo}}
	if _, err := repo.CreateMessage(ctx, message); !errors.Is(err, domain.ErrUploadExpired) {
		t.Fatalf("unreserved photo: %v", err)
	}
	items, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(items) != 0 {
		t.Fatalf("partial message published: %+v %v", items, err)
	}
	reserve(t, repo, photo)
	created, err := repo.CreateMessage(ctx, message)
	if err != nil || len(created.Photos) != 1 || created.Photos[0].CreatedAt.IsZero() {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	stored, err := repo.MessagePhoto(ctx, 1, message.ID, photo.ID)
	if err != nil || stored.StorageKey != photo.StorageKey || stored.UploadedBy != user.ID {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	for _, ids := range []struct {
		machine int64
		message string
		photo   string
	}{{2, message.ID, photo.ID}, {1, newID(), photo.ID}, {1, message.ID, newID()}} {
		if _, err := repo.MessagePhoto(ctx, ids.machine, ids.message, ids.photo); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("membership allowed %+v: %v", ids, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET active = FALSE WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckMachine(ctx, 1); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("inactive machine: %v", err)
	}
	if _, err := repo.MessagePhoto(ctx, 1, message.ID, photo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("inactive machine photo: %v", err)
	}
	if _, err := repo.CreateMessage(ctx, domain.Message{ID: newID(), MachineID: 1, Text: "note", Author: domain.MessageAuthor{ID: user.ID}}); !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("inactive create: %v", err)
	}
}

func TestIntegrationMessageMigrationPreservesExistingArchive(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `DROP VIEW machine_messages; DROP TABLE message_photos; DROP TABLE messages; DROP INDEX logbooks_machine_created_id_idx`); err != nil {
		t.Fatal(err)
	}
	photo := newPhoto(user.ID)
	reserve(t, repo, photo)
	book, err := repo.Create(ctx, newBook(user.ID, "2025-01-02", photo))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../migrations/00004_machine_messages.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, strings.Split(string(source), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	editMigration, err := os.ReadFile("../../migrations/00005_message_edits.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, strings.Split(string(editMigration), "-- +goose Down")[0]); err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(page) != 1 {
		t.Fatalf("migrated page=%+v err=%v", page, err)
	}
	message := page[0]
	if message.Text != "" || message.Author.ID != book.CreatedBy || message.Author.Name != user.Name || !message.CreatedAt.Equal(book.CreatedAt) || message.LegacyArchive == nil || message.LegacyArchive.ID != book.ID || message.LegacyArchive.LogDate != book.LogDate || message.LegacyArchive.Shift != book.Shift || len(message.Photos) != 1 || message.Photos[0].ID != book.Photos[0].ID || message.Photos[0].PageNumber != book.Photos[0].PageNumber || message.Photos[0].ContentType != book.Photos[0].ContentType || message.Photos[0].Size != book.Photos[0].Size || message.Photos[0].StorageKey != book.Photos[0].StorageKey || message.Photos[0].SHA256 != book.Photos[0].SHA256 || message.Photos[0].UploadedBy != book.Photos[0].UploadedBy || !message.Photos[0].CreatedAt.Equal(book.Photos[0].CreatedAt) {
		t.Fatalf("migration changed historical content: %+v", message)
	}
	if _, err := repo.Photo(ctx, book.ID, photo.ID); err != nil {
		t.Fatalf("historical photo lookup failed: %v", err)
	}
}

func TestIntegrationDeleteMessageRequiresSenderAndQueuesPhotos(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	admin := domain.User{ID: newID(), Username: "admin", Name: "Test admin", Role: "admin", Active: true, PasswordHash: "test-hash"}
	if err := repo.CreateUser(ctx, admin); err != nil {
		t.Fatal(err)
	}
	photo := newPhoto(user.ID)
	photo.PageNumber = 1
	reserve(t, repo, photo)
	message, err := repo.CreateMessage(ctx, domain.Message{ID: newID(), MachineID: 1, Text: "delete me", Author: domain.MessageAuthor{ID: user.ID}, Photos: []domain.Photo{photo}})
	if err != nil {
		t.Fatal(err)
	}
	archivePhoto := newPhoto(user.ID)
	reserve(t, repo, archivePhoto)
	archive, err := repo.Create(ctx, newBook(user.ID, "2026-09-22", archivePhoto))
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, admin.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin override: %v", err)
	}
	for _, tc := range []struct {
		machine int64
		message string
	}{{2, message.ID}, {99, message.ID}, {1, newID()}, {1, archive.ID}} {
		if err := repo.DeleteMessage(ctx, tc.machine, tc.message, user.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("delete %+v: %v", tc, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET active = FALSE WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("inactive machine delete: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE machines SET active = TRUE WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if keys, err := repo.PendingDeletions(ctx, 20); err != nil || len(keys) != 0 {
		t.Fatalf("rejected deletion queued photos: %v, %v", keys, err)
	}
	if _, err := repo.MessagePhoto(ctx, 1, message.ID, photo.ID); err != nil {
		t.Fatalf("rejected deletion removed photo: %v", err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(items) != 1 || items[0].ID != archive.ID {
		t.Fatalf("remaining messages=%+v err=%v", items, err)
	}
	if _, err := repo.MessagePhoto(ctx, 1, message.ID, photo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted photo lookup: %v", err)
	}
	if keys, err := repo.PendingDeletions(ctx, 20); err != nil || len(keys) != 1 || keys[0] != photo.StorageKey {
		t.Fatalf("deleted photo queue=%v err=%v", keys, err)
	}
	if _, err := repo.Photo(ctx, archive.ID, archivePhoto.ID); err != nil {
		t.Fatalf("legacy photo changed: %v", err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("repeat deletion: %v", err)
	}
}

func TestIntegrationDeleteMessageRollsBackWhenCleanupQueueFails(t *testing.T) {
	repo, pool, user := testRepository(t)
	ctx := context.Background()
	photo := newPhoto(user.ID)
	photo.PageNumber = 1
	reserve(t, repo, photo)
	message, err := repo.CreateMessage(ctx, domain.Message{ID: newID(), MachineID: 1, Text: "keep on failure", Author: domain.MessageAuthor{ID: user.ID}, Photos: []domain.Photo{photo}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE object_deletions ADD CONSTRAINT reject_cleanup CHECK (FALSE) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, user.ID); err == nil {
		t.Fatal("deletion succeeded despite cleanup queue failure")
	}
	items, err := repo.ListMessages(ctx, domain.MessageFilter{MachineID: 1, Limit: 20})
	if err != nil || len(items) != 1 || items[0].ID != message.ID || len(items[0].Photos) != 1 {
		t.Fatalf("rollback lost message or photo: %+v, %v", items, err)
	}
	stored, err := repo.MessagePhoto(ctx, 1, message.ID, photo.ID)
	if err != nil || stored.StorageKey != photo.StorageKey {
		t.Fatalf("rollback lost photo: %+v, %v", stored, err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE object_deletions DROP CONSTRAINT reject_cleanup`); err != nil {
		t.Fatal(err)
	}
	if keys, err := repo.PendingDeletions(ctx, 20); err != nil || len(keys) != 0 {
		t.Fatalf("failed deletion queued photos: %v, %v", keys, err)
	}
	if err := repo.DeleteMessage(ctx, 1, message.ID, user.ID); err != nil {
		t.Fatalf("retry deletion: %v", err)
	}
}
