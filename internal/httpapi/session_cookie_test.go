package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

type cookieSessionRepo struct {
	service.AuthRepository
	mu       sync.Mutex
	user     domain.User
	sessions map[string]domain.Session
}

func (r *cookieSessionRepo) UserByUsername(context.Context, string) (domain.User, error) {
	return r.user, nil
}
func (r *cookieSessionRepo) CreateSession(_ context.Context, s domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.TokenHash] = s
	return nil
}
func (r *cookieSessionRepo) Session(_ context.Context, hash string) (domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[hash]
	if !ok {
		return s, domain.ErrNotFound
	}
	return s, nil
}
func (r *cookieSessionRepo) DeleteSession(_ context.Context, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, hash)
	return nil
}

func TestPersistentCookieSessionHTTP(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	repo := &cookieSessionRepo{user: domain.User{ID: "operator", Username: "operator", Active: true, Role: "operator", PasswordHash: string(hash)}, sessions: make(map[string]domain.Session)}
	server := httptest.NewServer(New(nil, service.NewAuth(repo, 720*time.Hour), Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	request := func(client *http.Client, method, path, body, origin string) *http.Response {
		t.Helper()
		r, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		res, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}
	login := request(client, "POST", "/api/v1/auth/login", `{"username":"operator","password":"test-password"}`, server.URL)
	if login.StatusCode != 200 {
		t.Fatalf("login: %d", login.StatusCode)
	}
	var result service.LoginResult
	if err := json.NewDecoder(login.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	cookies := login.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies: %v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Value != result.AccessToken || cookie.Path != "/api/v1" || !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge < 2591990 || cookie.MaxAge > 2592000 || !cookie.Expires.Equal(result.ExpiresAt.Truncate(time.Second)) {
		t.Fatalf("wrong cookie attributes: %+v", cookie)
	}
	// Restore persisted cookie into a fresh browser client, without bearer headers.
	restoredJar, _ := cookiejar.New(nil)
	u, _ := url.Parse(server.URL + "/api/v1/auth/me")
	restoredJar.SetCookies(u, cookies)
	restored := &http.Client{Jar: restoredJar}
	if res := request(restored, "GET", "/api/v1/auth/me", "", ""); res.StatusCode != 200 {
		t.Fatalf("restore: %d", res.StatusCode)
	}
	for _, origin := range []string{"", "https://attacker.test"} {
		if res := request(restored, "POST", "/api/v1/auth/logout", "", origin); res.StatusCode != 403 {
			t.Fatalf("CSRF %q: %d", origin, res.StatusCode)
		}
	}
	logout := request(restored, "POST", "/api/v1/auth/logout", "", server.URL)
	if logout.StatusCode != 204 || len(logout.Cookies()) != 1 || logout.Cookies()[0].MaxAge != -1 || logout.Cookies()[0].Path != "/api/v1" {
		t.Fatalf("logout: %d %v", logout.StatusCode, logout.Cookies())
	}
	restoredJar.SetCookies(u, cookies)
	if res := request(restored, "GET", "/api/v1/auth/me", "", ""); res.StatusCode != 401 {
		t.Fatalf("revoked replay: %d", res.StatusCode)
	}
	login = request(client, "POST", "/api/v1/auth/login", `{"username":"operator","password":"test-password"}`, server.URL)
	if login.StatusCode != 200 {
		t.Fatalf("second login: %d", login.StatusCode)
	}
	repo.mu.Lock()
	for key, session := range repo.sessions {
		session.ExpiresAt = time.Now().Add(-time.Second)
		repo.sessions[key] = session
	}
	repo.mu.Unlock()
	if res := request(client, "GET", "/api/v1/auth/me", "", ""); res.StatusCode != 401 {
		t.Fatalf("expired cookie: %d", res.StatusCode)
	}
}

func TestCookieOriginAndBearerRules(t *testing.T) {
	s := &Server{opts: Options{SessionCookieSecure: true, AllowedOrigins: []string{"https://trusted.test"}}}
	for _, tc := range []struct {
		origin, referer string
		want            bool
	}{
		{"https://example.com", "", true}, {"http://example.com", "", false},
		{"https://trusted.test", "", true}, {"", "https://example.com/page", true},
		{"", "", false}, {"null", "https://example.com/page", false},
		{"https://evil.test", "https://example.com/page", false}, {"", "https://example.com.evil.test/page", false},
	} {
		r := httptest.NewRequest("POST", "http://example.com/api/v1/auth/logout", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Referer", tc.referer)
		if got := s.trustedRequestOrigin(r); got != tc.want {
			t.Fatalf("%+v: %v", tc, got)
		}
	}
	w := httptest.NewRecorder()
	s.setSessionCookie(w, "token", time.Now().Add(720*time.Hour))
	if !w.Result().Cookies()[0].Secure {
		t.Fatal("production cookie lacks Secure")
	}
	r := httptest.NewRequest("POST", "/api/v1/test", nil)
	authorized(r)
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	h := &Server{auth: service.NewAuth(sessionRepo{role: "operator"}, time.Hour)}
	w = httptest.NewRecorder()
	h.authenticated(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("bearer must not require Origin: %d", w.Code)
	}
}
