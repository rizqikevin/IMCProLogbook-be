package httpapi

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

type socketRepo struct {
	service.Repository
	mu       sync.Mutex
	messages []domain.Message
}

func (r *socketRepo) CheckMachine(context.Context, int64) error { return nil }
func (r *socketRepo) ListMessages(_ context.Context, f domain.MessageFilter) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []domain.Message{}
	for _, m := range r.messages {
		if m.MachineID == f.MachineID {
			items = append(items, m)
		}
	}
	return items, nil
}
func (r *socketRepo) CreateMessage(_ context.Context, m domain.Message) (domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, m)
	return m, nil
}

func TestMessageSocketAuthIsolationAndSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repo := &socketRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := httptest.NewServer(New(service.NewLogbooks(repo, nil, service.Limits{}, logger), service.NewAuth(sessionRepo{role: "operator"}, time.Hour), Options{Logger: logger, SocketContext: ctx}))
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/machines/"
	dial := func(machine, token string) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, url+machine+"/messages/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.CloseNow() })
		if err := wsjson.Write(ctx, conn, socketCommand{Type: "auth", Token: token}); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	read := func(conn *websocket.Conn) map[string]any {
		t.Helper()
		var event map[string]any
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatal(err)
		}
		return event
	}
	bad := dial("1", "invalid")
	var ignored any
	if err := wsjson.Read(ctx, bad, &ignored); websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("invalid auth: %v", err)
	}
	_, response, err := websocket.Dial(ctx, url+"1/messages/ws", &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://attacker.invalid"}}})
	if err == nil || response == nil || response.StatusCode != 403 {
		t.Fatal("foreign origin accepted")
	}
	token := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	sender, peer, other := dial("1", token), dial("1", token), dial("2", token)
	for _, conn := range []*websocket.Conn{sender, peer, other} {
		if read(conn)["type"] != "ready" {
			t.Fatal("missing ready")
		}
	}
	if err := wsjson.Write(ctx, sender, socketCommand{Type: "send", RequestID: "one", Text: "Mesin berhenti"}); err != nil {
		t.Fatal(err)
	}
	ack := read(sender)
	if ack["type"] != "ack" || ack["request_id"] != "one" {
		t.Fatalf("ack: %#v", ack)
	}
	for _, conn := range []*websocket.Conn{sender, peer} {
		event := read(conn)
		if event["type"] != "created" || event["machine_id"] != float64(1) {
			t.Fatalf("event: %#v", event)
		}
		message := event["message"].(map[string]any)
		if message["text"] != "Mesin berhenti" {
			t.Fatalf("message: %#v", message)
		}
	}
	repo.mu.Lock()
	count := len(repo.messages)
	repo.mu.Unlock()
	if count != 1 {
		t.Fatalf("persisted %d messages", count)
	}
	isolated, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	defer stop()
	if wsjson.Read(isolated, other, &ignored) == nil {
		t.Fatal("message leaked into another machine")
	}
}
