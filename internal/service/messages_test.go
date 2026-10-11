package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"machine-logbook/internal/domain"
)

type messageRepoStub struct {
	*logbookRepoStub
	checkMachine  func(context.Context, int64) error
	createMessage func(context.Context, domain.Message) (domain.Message, error)
	listMessages  func(context.Context, domain.MessageFilter) ([]domain.Message, error)
}

func (r *messageRepoStub) CheckMachine(ctx context.Context, id int64) error {
	if r.checkMachine != nil {
		return r.checkMachine(ctx, id)
	}
	return nil
}
func (r *messageRepoStub) CreateMessage(ctx context.Context, message domain.Message) (domain.Message, error) {
	return r.createMessage(ctx, message)
}
func (r *messageRepoStub) ListMessages(ctx context.Context, filter domain.MessageFilter) ([]domain.Message, error) {
	return r.listMessages(ctx, filter)
}

func TestCreateMessageAcceptsTextPhotosOrBoth(t *testing.T) {
	for _, kind := range []string{"text", "photos", "both", "unicode-limit"} {
		t.Run(kind, func(t *testing.T) {
			text := "  Catatan mesin\nbaris kedua  "
			var uploads []Upload
			if kind == "photos" || kind == "both" {
				uploads = []Upload{uploadBytes(imageBytes(t, "png", 2, 2))}
			}
			if kind == "photos" {
				text = ""
			}
			if kind == "unicode-limit" {
				text = strings.Repeat("界", 20000)
			}
			var events []string
			repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{
				reserve: func(context.Context, []string, time.Time) error {
					events = append(events, "reserve")
					return nil
				},
			}, createMessage: func(_ context.Context, message domain.Message) (domain.Message, error) {
				events = append(events, "publish")
				if message.Text != text || message.MachineID != 2 || message.Author.ID != testUserID || message.Author.Name != "Operator" || message.CreatedAt.IsZero() || !validUUID(message.ID) || message.LegacyArchive != nil {
					t.Fatalf("incorrect message: %+v", message)
				}
				return message, nil
			}}
			store := &storeStub{put: func(_ context.Context, key string, _ io.Reader, _ int64, _ string) error {
				if !strings.HasPrefix(key, "messages/") {
					t.Fatalf("wrong storage key %s", key)
				}
				events = append(events, "put")
				return nil
			}}
			message, err := testLogbooks(repo, store).CreateMessage(context.Background(), domain.User{ID: testUserID, Name: "Operator"}, 2, text, uploads)
			if err != nil || message.Photos == nil || len(message.Photos) != len(uploads) {
				t.Fatalf("message=%+v err=%v", message, err)
			}
			if len(uploads) > 0 {
				if strings.Join(events, ",") != "reserve,put,publish" || !strings.HasPrefix(message.Photos[0].URL, "/api/v1/machines/2/messages/") {
					t.Fatalf("events=%v photos=%+v", events, message.Photos)
				}
			} else if strings.Join(events, ",") != "publish" {
				t.Fatalf("text-only message used storage: %v", events)
			}
		})
	}
}

func TestCreateMessageRejectsInvalidContentBeforePublishing(t *testing.T) {
	for _, text := range []string{"", " \n\t\u2003", strings.Repeat("界", 20001), "a\x00b", string([]byte{0xff})} {
		repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{}}
		_, err := testLogbooks(repo, &storeStub{}).CreateMessage(context.Background(), domain.User{ID: testUserID}, 1, text, nil)
		var validation *domain.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("expected validation error, got %v", err)
		}
	}
	repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{}, checkMachine: func(context.Context, int64) error { return domain.ErrInvalidReference }}
	_, err := testLogbooks(repo, &storeStub{}).CreateMessage(context.Background(), domain.User{ID: testUserID}, 99, "note", nil)
	if !errors.Is(err, domain.ErrInvalidReference) {
		t.Fatalf("inactive machine: %v", err)
	}
}

func TestMessagePageReturnsChronologicalItemsAndOlderCursor(t *testing.T) {
	stamp := time.Date(2026, 10, 1, 8, 0, 0, 123000, time.UTC)
	ids := []string{"00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000001"}
	repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{}, listMessages: func(_ context.Context, filter domain.MessageFilter) ([]domain.Message, error) {
		if filter.MachineID != 1 || filter.Limit != 3 {
			t.Fatalf("wrong filter %+v", filter)
		}
		return []domain.Message{
			{ID: ids[0], MachineID: 1, CreatedAt: stamp},
			{ID: ids[1], MachineID: 1, CreatedAt: stamp, LegacyArchive: &domain.LegacyArchive{ID: ids[1]}, Photos: []domain.Photo{{ID: testBookID}}},
			{ID: ids[2], MachineID: 1, CreatedAt: stamp},
		}, nil
	}}
	svc := testLogbooks(repo, &storeStub{})
	page, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Limit: 2})
	if err != nil || len(page.Items) != 2 || page.Items[0].ID != ids[1] || page.Items[1].ID != ids[0] || page.Items[1].Photos == nil {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].Text != "" || page.Items[0].Photos[0].URL != photoURL(ids[1], testBookID) {
		t.Fatalf("legacy changed: %+v", page.Items[0])
	}
	cursor, err := ParseMessageCursor(page.NextCursor)
	if err != nil || cursor.ID != ids[1] || cursor.MachineID != 1 || !cursor.CreatedAt.Equal(stamp) {
		t.Fatalf("cursor=%+v err=%v", cursor, err)
	}
	if _, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 2, Cursor: cursor}); err == nil {
		t.Fatal("accepted cursor for another machine")
	}
}

func TestMessageCursorRejectsMalformedValues(t *testing.T) {
	for _, raw := range []string{"!", strings.Repeat("a", 513), "e30", "bnVsbA"} {
		if _, err := ParseMessageCursor(raw); err == nil {
			t.Fatalf("accepted cursor %q", raw)
		}
	}
}

func TestMessageSearchValidationAndCursorBinding(t *testing.T) {
	stamp := time.Now().UTC()
	repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{}, listMessages: func(_ context.Context, filter domain.MessageFilter) ([]domain.Message, error) {
		if filter.Query != strings.TrimSpace(filter.Query) {
			t.Fatalf("query not trimmed: %q", filter.Query)
		}
		return []domain.Message{{ID: testBookID, MachineID: 1, CreatedAt: stamp}, {ID: testUserID, MachineID: 1, CreatedAt: stamp}}, nil
	}}
	svc := testLogbooks(repo, &storeStub{})
	for _, query := range []string{"needle", strings.Repeat("界", 200)} {
		page, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Limit: 1, Query: " \t" + query + "\n"})
		if err != nil || page.NextCursor == "" || len(page.NextCursor) > 512 {
			t.Fatalf("page=%+v err=%v", page, err)
		}
		cursor, err := ParseMessageCursor(page.NextCursor)
		if err != nil || len(cursor.QueryHash) != 64 {
			t.Fatalf("cursor=%+v err=%v", cursor, err)
		}
		if _, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Query: " " + query + " ", Cursor: cursor}); err != nil {
			t.Fatalf("same normalized query rejected: %v", err)
		}
		for _, changed := range []string{"", "different"} {
			if _, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Query: changed, Cursor: cursor}); err == nil {
				t.Fatalf("accepted changed query %q", changed)
			}
		}
	}
	for _, query := range []string{"", " \n\t\u2003"} {
		page, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Limit: 1, Query: query})
		if err != nil {
			t.Fatal(err)
		}
		cursor, err := ParseMessageCursor(page.NextCursor)
		if err != nil || cursor.QueryHash != "" {
			t.Fatalf("blank query cursor=%+v err=%v", cursor, err)
		}
		if _, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Query: "needle", Cursor: cursor}); err == nil {
			t.Fatal("accepted timeline cursor in search")
		}
	}
	repo.listMessages = func(context.Context, domain.MessageFilter) ([]domain.Message, error) {
		t.Fatal("invalid query reached repository")
		return nil, nil
	}
	for _, query := range []string{strings.Repeat("界", 201), "a\x00b", string([]byte{0xff})} {
		_, err := svc.ListMessages(context.Background(), domain.MessageFilter{MachineID: 1, Query: query})
		var validation *domain.ValidationError
		if !errors.As(err, &validation) {
			t.Fatalf("query %q: %v", query, err)
		}
	}
}

func TestMessageUploadFailureNeverPublishes(t *testing.T) {
	for _, corrupt := range []bool{true, false} {
		repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{
			reserve: func(context.Context, []string, time.Time) error { return nil },
		}, createMessage: func(context.Context, domain.Message) (domain.Message, error) {
			t.Fatal("published failed upload")
			return domain.Message{}, nil
		}}
		store := &storeStub{put: func(context.Context, string, io.Reader, int64, string) error {
			if corrupt {
				t.Fatal("stored before validating whole batch")
			}
			return errors.New("storage unavailable")
		}}
		uploads := []Upload{uploadBytes(imageBytes(t, "png", 2, 2))}
		if corrupt {
			uploads = append(uploads, uploadBytes([]byte("not an image")))
		}
		if _, err := testLogbooks(repo, store).CreateMessage(context.Background(), domain.User{ID: testUserID}, 1, "note", uploads); err == nil {
			t.Fatal("failed upload accepted")
		}
	}
}

func TestCreateMessageNormalizesCRLFBeforeValidationAndStorage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  string
	}{
		{"whitespace", "  Baris satu\r\n\tBaris dua\nBaris tiga\rakhir  ", "  Baris satu\n\tBaris dua\nBaris tiga\rakhir  "},
		{"normalized limit", "x" + strings.Repeat("\r\n", 19999), "x" + strings.Repeat("\n", 19999)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &messageRepoStub{logbookRepoStub: &logbookRepoStub{}, createMessage: func(_ context.Context, message domain.Message) (domain.Message, error) {
				if message.Text != tc.want {
					t.Fatalf("stored text differs from canonical LF text")
				}
				return message, nil
			}}
			message, err := testLogbooks(repo, &storeStub{}).CreateMessage(context.Background(), domain.User{ID: testUserID}, 1, tc.input, nil)
			if err != nil || message.Text != tc.want {
				t.Fatalf("canonical LF response mismatch: %v", err)
			}
		})
	}
}
