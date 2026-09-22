package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

type sessionRepo struct {
	service.AuthRepository
	role string
}

func (r sessionRepo) Session(context.Context, string) (domain.Session, error) {
	return domain.Session{User: domain.User{ID: "06f28606-ff86-49f3-bf98-7ffab8c3d2a4", Username: "operator", Role: r.role, Active: true}, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func testHandler(role string, opts Options) http.Handler {
	opts.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(nil, service.NewAuth(sessionRepo{role: role}, time.Hour), opts)
}
func authorized(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
}

func TestProtectedEndpointsRejectMissingToken(t *testing.T) {
	h := testHandler("operator", Options{})
	for _, tc := range []struct{ method, path string }{{"GET", "/api/v1/machines"}, {"GET", "/api/v1/shifts"}, {"GET", "/api/v1/logbooks"}, {"POST", "/api/v1/logbooks"}, {"GET", "/api/v1/logbooks/x/photos/y"}, {"DELETE", "/api/v1/logbooks/x"}, {"GET", "/api/v1/auth/me"}, {"POST", "/api/v1/auth/logout"}} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != 401 {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
			if w.Header().Get("X-Request-ID") == "" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing response protection headers")
			}
		})
	}
}
func TestOperatorCannotDelete(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/api/v1/logbooks/x", nil)
	authorized(r)
	testHandler("operator", Options{}).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("got %d", w.Code)
	}
}
func TestCORSAndHealth(t *testing.T) {
	h := testHandler("operator", Options{AllowedOrigins: []string{"https://logbook.test"}, Ready: func(context.Context) error { return errors.New("private DB error") }})
	for _, tc := range []struct {
		method, path, origin, requestMethod string
		status                              int
	}{{"GET", "/health/live", "", "", 200}, {"GET", "/health/ready", "", "", 503}, {"GET", "/health/live", "https://attacker.test", "", 403}, {"OPTIONS", "/api/v1/logbooks", "https://logbook.test", "POST", 204}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Access-Control-Request-Method", tc.requestMethod)
		h.ServeHTTP(w, r)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "private DB") {
			t.Fatalf("got %d %s", w.Code, w.Body)
		}
		if tc.status == 204 && w.Header().Get("Access-Control-Allow-Origin") != tc.origin {
			t.Fatal("missing allowed origin")
		}
	}
}
func TestLoginRejectsMalformedBodyAndRateLimits(t *testing.T) {
	h := testHandler("operator", Options{})
	for _, tc := range []struct {
		body, ctype string
		status      int
	}{{"{}", "text/plain", 415}, {"{", "application/json", 400}, {`{"unexpected":1}`, "application/json", 400}, {`{} {}`, "application/json", 400}, {`{"password":"` + strings.Repeat("a", 5000) + `"}`, "application/json", 413}} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", tc.ctype)
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("got %d %s", w.Code, w.Body)
		}
	}
	for i := 0; i < 6; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("{"))
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		if i == 5 && w.Code != 429 {
			t.Fatalf("expected rate limiting: %d", w.Code)
		}
	}
}
func TestMalformedMultipartAndSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name      string
		fields    [][2]string
		fileField string
		max       int64
		status    int
	}{
		{"duplicate", [][2]string{{"machine_id", "1"}, {"machine_id", "2"}, {"shift_id", "1"}, {"log_date", "2026-09-22"}}, "photos", 0, 400},
		{"missing", [][2]string{{"machine_id", "1"}, {"shift_id", "1"}}, "photos", 0, 400},
		{"unknown file", [][2]string{{"machine_id", "1"}, {"shift_id", "1"}, {"log_date", "2026-09-22"}}, "file", 0, 400},
		{"request limit", [][2]string{{"machine_id", "1"}, {"shift_id", "1"}, {"log_date", "2026-09-22"}}, "photos", 10, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			mw := multipart.NewWriter(&body)
			for _, field := range tc.fields {
				_ = mw.WriteField(field[0], field[1])
			}
			f, _ := mw.CreateFormFile(tc.fileField, "page.jpg")
			_, _ = f.Write([]byte("image"))
			_ = mw.Close()
			r := httptest.NewRequest("POST", "/api/v1/logbooks", &body)
			r.Header.Set("Content-Type", mw.FormDataContentType())
			authorized(r)
			w := httptest.NewRecorder()
			testHandler("operator", Options{MaxRequestBytes: tc.max}).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
}
func TestInvalidQueryParameters(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=101", "machine_id=-1", "machine_id=1&machine_id=2", "unknown=x", "shift_id=", "date_from=%ZZ", "limit=1;shift_id=1"} {
		r := httptest.NewRequest("GET", "/api/v1/logbooks?"+query, nil)
		authorized(r)
		w := httptest.NewRecorder()
		testHandler("operator", Options{}).ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("query %q: %d %s", query, w.Code, w.Body)
		}
	}
}
func TestUploadCapacityAndRecovery(t *testing.T) {
	s := &Server{uploads: make(chan struct{}, 1), opts: Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RequestTimeout: time.Minute}}
	s.uploads <- struct{}{}
	w := httptest.NewRecorder()
	s.upload(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("busy upload was executed") })).ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
	if w.Code != 503 {
		t.Fatalf("got %d", w.Code)
	}
	<-s.uploads
	w = httptest.NewRecorder()
	s.middleware(s.upload(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") }))).ServeHTTP(w, httptest.NewRequest("POST", "/", nil))
	if w.Code != 500 || len(s.uploads) != 0 {
		t.Fatalf("panic recovery failed: status %d slots %d", w.Code, len(s.uploads))
	}
}
func TestLoginLimiterExpiresAndBoundsMemory(t *testing.T) {
	l := newLoginLimiter()
	now := time.Now()
	for range 10 {
		if !l.allow("127.0.0.1:9000", now) {
			t.Fatal("rejected initial attempt")
		}
	}
	if l.allow("127.0.0.1:1234", now) {
		t.Fatal("port variation bypassed limit")
	}
	if !l.allow("127.0.0.1:1234", now.Add(time.Minute)) {
		t.Fatal("window did not reset")
	}
	for i := 0; i < 4096; i++ {
		l.windows[string(rune(i))] = loginWindow{started: now, attempts: 1}
	}
	if l.allow("new-address", now) {
		t.Fatal("allowed unbounded map growth")
	}
	if !l.allow("new-address", now.Add(2*time.Minute)) {
		t.Fatal("did not evict expired keys")
	}
}
