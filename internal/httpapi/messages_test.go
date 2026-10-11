package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

type messageHTTPRepo struct {
	service.Repository
	messages   []domain.Message
	lastFilter domain.MessageFilter
}

func (r *messageHTTPRepo) CheckMachine(_ context.Context, id int64) error {
	if id != 1 {
		return domain.ErrInvalidReference
	}
	return nil
}
func (r *messageHTTPRepo) CreateMessage(_ context.Context, message domain.Message) (domain.Message, error) {
	r.messages = append(r.messages, message)
	return message, nil
}
func (r *messageHTTPRepo) ListMessages(_ context.Context, filter domain.MessageFilter) ([]domain.Message, error) {
	r.lastFilter = filter
	items := make([]domain.Message, 0)
	for i := len(r.messages) - 1; i >= 0 && len(items) < filter.Limit; i-- {
		items = append(items, r.messages[i])
	}
	return items, nil
}

func TestMessageHTTPContract(t *testing.T) {
	repo := &messageHTTPRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(service.NewLogbooks(repo, nil, service.Limits{}, logger), service.NewAuth(sessionRepo{role: "operator"}, time.Hour), Options{Logger: logger})
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/machines/1/messages", nil))
		if w.Code != 401 {
			t.Fatalf("unauthenticated %s: %d", method, w.Code)
		}
	}
	for _, tc := range []struct {
		fields [][2]string
		status int
	}{
		{[][2]string{{"text", "  Mesin berhenti\nperiksa motor  "}}, 201},
		{nil, 400},
		{[][2]string{{"text", "\n \t"}}, 400},
		{[][2]string{{"text", "a"}, {"text", "b"}}, 400},
		{[][2]string{{"text", "a"}, {"shift_id", "1"}}, 400},
		{[][2]string{{"text", "a"}, {"author", "spoof"}}, 400},
		{[][2]string{{"text", strings.Repeat("界", 20001)}}, 400},
	} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		for _, field := range tc.fields {
			if err := form.WriteField(field[0], field[1]); err != nil {
				t.Fatal(err)
			}
		}
		if err := form.Close(); err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/api/v1/machines/1/messages", &body)
		r.Header.Set("Content-Type", form.FormDataContentType())
		authorized(r)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("POST got %d: %s", w.Code, w.Body)
		}
		if tc.status == 201 {
			var message domain.Message
			if err := json.Unmarshal(w.Body.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			if message.Text != tc.fields[0][1] || message.Author.ID != "06f28606-ff86-49f3-bf98-7ffab8c3d2a4" || message.Photos == nil || message.CreatedAt.IsZero() || message.MachineID != 1 {
				t.Fatalf("bad message %+v", message)
			}
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/api/v1/machines/1/messages?cursor=&limit=20", 200},
		{"/api/v1/machines/2/messages", 400},
		{"/api/v1/machines/no/messages", 400},
		{"/api/v1/machines/1/messages?limit=101", 400},
		{"/api/v1/machines/1/messages?limit=0", 400},
		{"/api/v1/machines/1/messages?limit=", 400},
		{"/api/v1/machines/1/messages?cursor=bad", 400},
		{"/api/v1/machines/1/messages?limit=1&limit=2", 400},
		{"/api/v1/machines/1/messages?shift_id=1", 400},
		{"/api/v1/machines/1/messages?q=", 200},
		{"/api/v1/machines/1/messages?q=%20%09", 200},
		{"/api/v1/machines/1/messages?q=" + url.QueryEscape(strings.Repeat("界", 200)), 200},
		{"/api/v1/machines/1/messages?q=" + url.QueryEscape(strings.Repeat("界", 201)), 400},
		{"/api/v1/machines/1/messages?q=%FF", 400},
		{"/api/v1/machines/1/messages?q=%00", 400},
		{"/api/v1/machines/1/messages?q=a&q=b", 400},
	} {
		r := httptest.NewRequest("GET", tc.path, nil)
		authorized(r)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s got %d: %s", tc.path, w.Code, w.Body)
		}
		if tc.status == 200 {
			var page domain.MessagePage
			if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.NextCursor != "" {
				t.Fatalf("page=%+v err=%v", page, err)
			}
		}
	}
	r := httptest.NewRequest("GET", "/api/v1/machines/1/messages?q=%20Motor%20%20panas%20", nil)
	authorized(r)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || repo.lastFilter.Query != "Motor  panas" {
		t.Fatalf("search forwarding: status=%d filter=%+v", w.Code, repo.lastFilter)
	}
}
