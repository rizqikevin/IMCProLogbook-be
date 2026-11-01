package service

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"machine-logbook/internal/domain"
)

type pushWorkerRepo struct {
	PushRepository
	subscriptions []domain.PushSubscription
	deleted       []string
	lookupErr     error
	deleteErr     error
	finished      chan struct{}
}

func (r *pushWorkerRepo) PushSubscriptions(_ context.Context, userID string) ([]domain.PushSubscription, error) {
	if userID == "barrier" {
		close(r.finished)
		return nil, nil
	}
	return r.subscriptions, r.lookupErr
}

func (r *pushWorkerRepo) DeletePushSubscription(_ context.Context, endpoint string) error {
	r.deleted = append(r.deleted, endpoint)
	return r.deleteErr
}

func TestPushWorkerDelivery(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		networkErr bool
		rejected   string
		lookupErr  bool
		deleteErr  bool
		wantLog    string
	}{
		{name: "accepted", status: 201},
		{name: "redirect", status: 302, wantLog: "push provider rejected delivery"},
		{name: "unauthorized", status: 401, wantLog: "push provider rejected delivery"},
		{name: "forbidden", status: 403, wantLog: "push provider rejected delivery"},
		{name: "missing", status: 404, wantLog: "push provider rejected delivery"},
		{name: "gone", status: 410, wantLog: "push provider rejected delivery"},
		{name: "rate limited", status: 429, wantLog: "push provider rejected delivery"},
		{name: "provider failure", status: 503, wantLog: "push provider rejected delivery"},
		{name: "network failure", networkErr: true, wantLog: "push delivery failed"},
		{name: "private endpoint", rejected: "https://127.0.0.2/secret-endpoint", wantLog: "push endpoint rejected"},
		{name: "invalid scheme", rejected: "http://127.0.0.1/secret-endpoint", wantLog: "push endpoint rejected"},
		{name: "DNS failure", rejected: "https://push-worker-test.invalid/secret-endpoint", wantLog: "push endpoint rejected"},
		{name: "lookup failure", lookupErr: true, wantLog: "push subscription lookup failed"},
		{name: "cleanup failure", status: 410, deleteErr: true, wantLog: "push subscription cleanup failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := ecdh.P256().GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			auth := make([]byte, 16)
			if _, err := rand.Read(auth); err != nil {
				t.Fatal(err)
			}
			requests := make(chan []byte, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if r.Method != http.MethodPost || r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("Authorization") == "" {
					t.Error("missing encrypted webpush request headers")
				}
				requests <- body
				w.Header().Set("Location", "/secret-redirect")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, "secret-provider-body")
			}))
			defer server.Close()
			u, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			sub := domain.PushSubscription{UserID: "recipient", Endpoint: server.URL + "/secret-endpoint?token=secret-query", P256DH: base64.RawURLEncoding.EncodeToString(receiver.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(auth)}
			if tc.rejected != "" {
				sub.Endpoint = tc.rejected
			}
			if tc.networkErr {
				server.Close()
			}
			repo := &pushWorkerRepo{subscriptions: []domain.PushSubscription{sub}, finished: make(chan struct{})}
			if tc.lookupErr {
				repo.lookupErr = errors.New("secret-repository-error")
			}
			if tc.deleteErr {
				repo.deleteErr = errors.New("secret-repository-error")
			}
			var logs bytes.Buffer
			previousLogger := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer slog.SetDefault(previousLogger)
			push := NewPushService(repo, PushConfig{VAPIDPublicKey: publicKey, VAPIDPrivateKey: privateKey, Subscriber: "mailto:push@example.com", TrustedHosts: []string{u.Hostname()}})
			defer push.Close()
			client := server.Client()
			client.Timeout = time.Second
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			push.options.HTTPClient = client
			if err := push.Enqueue("recipient", map[string]string{"message": "secret-payload"}); err != nil {
				t.Fatal(err)
			}
			if err := push.Enqueue("barrier", nil); err != nil {
				t.Fatal(err)
			}
			select {
			case <-repo.finished:
			case <-time.After(10 * time.Second):
				t.Fatal("worker did not finish")
			}
			push.Close()
			wantDelete := tc.status == 404 || tc.status == 410
			if (len(repo.deleted) == 1) != wantDelete || len(repo.deleted) > 1 {
				t.Fatalf("delete count = %d, want deletion %v", len(repo.deleted), wantDelete)
			}
			if wantDelete && repo.deleted[0] != sub.Endpoint {
				t.Error("deleted wrong subscription")
			}
			wantRequest := !tc.networkErr && tc.rejected == "" && !tc.lookupErr
			select {
			case body := <-requests:
				if !wantRequest || len(body) == 0 || bytes.Contains(body, []byte("secret-payload")) {
					t.Error("unexpected request or unencrypted payload")
				}
			default:
				if wantRequest {
					t.Error("provider received no webpush request")
				}
			}
			found := false
			decoder := json.NewDecoder(strings.NewReader(logs.String()))
			for decoder.More() {
				var entry map[string]any
				if err := decoder.Decode(&entry); err != nil {
					t.Fatal(err)
				}
				if entry["msg"] == tc.wantLog {
					found = true
					if !tc.lookupErr {
						endpoint, _ := url.Parse(sub.Endpoint)
						if entry["provider"] != endpoint.Hostname() {
							t.Error("missing provider host")
						}
					}
					if tc.status >= 300 && entry["status"] != float64(tc.status) {
						t.Error("missing provider status")
					}
				}
			}
			if tc.wantLog != "" && !found {
				t.Errorf("missing diagnostic %q", tc.wantLog)
			}
			if tc.wantLog == "" && logs.Len() != 0 {
				t.Error("successful delivery logged as failure")
			}
			for _, secret := range []string{sub.Endpoint, sub.Auth, sub.P256DH, privateKey, publicKey, "secret-endpoint", "secret-query", "secret-payload", "secret-provider-body", "secret-repository-error", "secret-redirect"} {
				if strings.Contains(logs.String(), secret) {
					t.Error("push logs contain secret data")
				}
			}
		})
	}
}
