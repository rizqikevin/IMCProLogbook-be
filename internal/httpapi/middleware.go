package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"machine-logbook/internal/domain"
)

type contextKey int

const (
	requestIDKey contextKey = iota
	userKey
)

func requestID(r *http.Request) string { id, _ := r.Context().Value(requestIDKey).(string); return id }
func currentUser(r *http.Request) domain.User {
	user, _ := r.Context().Value(userKey).(domain.User)
	return user
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx, cancel := context.WithTimeout(r.Context(), s.opts.RequestTimeout)
		defer cancel()
		ctx = context.WithValue(ctx, requestIDKey, uuid.NewString())
		r = r.WithContext(ctx)
		out := &statusWriter{ResponseWriter: w}
		out.Header().Set("X-Request-ID", requestID(r))
		out.Header().Set("X-Content-Type-Options", "nosniff")
		out.Header().Set("Cache-Control", "no-store")
		out.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/docs") {
			// Allow CDN assets for Swagger UI.
			out.Header().Set("Content-Security-Policy",
				"default-src 'none'; script-src 'unsafe-inline' https://unpkg.com; style-src 'unsafe-inline' https://unpkg.com; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		} else {
			out.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				s.opts.Logger.Error("request panicked", "request_id", requestID(r), "panic", recovered, "stack", string(debug.Stack()))
				if out.status == 0 {
					problem(out, r, 500, "internal_error", "an internal error occurred")
				}
			}
			status := out.status
			if status == 0 {
				status = http.StatusOK
			}
			s.opts.Logger.Info("http request", "request_id", requestID(r), "method", r.Method, "path", r.URL.Path, "status", status, "bytes", out.bytes, "duration_ms", time.Since(start).Milliseconds())
		}()
		out.Header().Add("Vary", "Origin")
		if origin := r.Header.Get("Origin"); origin != "" {
			sameOrigin := origin == "http://"+r.Host || origin == "https://"+r.Host
			allowed := sameOrigin
			if !allowed {
				for _, v := range s.opts.AllowedOrigins {
					if v == origin {
						allowed = true
						break
					}
				}
			}
			if !allowed {
				problem(out, r, 403, "origin_not_allowed", "origin is not allowed")
				return
			}
			if !sameOrigin {
				out.Header().Set("Access-Control-Allow-Origin", origin)
				out.Header().Set("Access-Control-Allow-Credentials", "true")
				out.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Location, ETag")
			}
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if !sameOrigin {
					out.Header().Add("Vary", "Access-Control-Request-Method")
					out.Header().Add("Vary", "Access-Control-Request-Headers")
					out.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
					out.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					out.Header().Set("Access-Control-Max-Age", "600")
				}
				out.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(out, r)
	})
}

func (s *Server) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, cookieAuth := sessionToken(r)
		if token == "" {
			s.failure(w, r, domain.ErrUnauthorized)
			return
		}
		if cookieAuth && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions && !s.trustedRequestOrigin(r) {
			problem(w, r, http.StatusForbidden, "origin_not_allowed", "trusted Origin or Referer required")
			return
		}
		user, err := s.auth.Authenticate(r.Context(), token)
		if err != nil {
			s.failure(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	})
}

const sessionCookieName = "logbook_session"

func sessionToken(r *http.Request) (string, bool) {
	if r.Header.Get("Authorization") != "" {
		return bearer(r), false
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		return cookie.Value, true
	}
	return "", false
}

func (s *Server) trustedRequestOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		u, err := url.Parse(r.Referer())
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return false
		}
		origin = u.Scheme + "://" + u.Host
	}
	scheme := "http"
	if s.opts.SessionCookieSecure || r.TLS != nil {
		scheme = "https"
	}
	if origin == scheme+"://"+r.Host {
		return true
	}
	for _, allowed := range s.opts.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if token == "" {
		maxAge = -1
	} else if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookieName, Value: token, Path: "/api/v1", HttpOnly: true, Secure: s.opts.SessionCookieSecure, SameSite: http.SameSiteStrictMode, Expires: expires, MaxAge: maxAge})
}
func (s *Server) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r).Role != "admin" {
			problem(w, r, 403, "forbidden", "admin role required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Server) upload(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case s.uploads <- struct{}{}:
			defer func() { <-s.uploads }()
		default:
			w.Header().Set("Retry-After", "2")
			problem(w, r, 503, "upload_busy", "upload capacity reached, retry shortly")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type loginWindow struct {
	started  time.Time
	attempts int
}
type loginLimiter struct {
	mu      sync.Mutex
	windows map[string]loginWindow
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{windows: make(map[string]loginWindow)} }
func (l *loginLimiter) allow(remote string, now time.Time) bool {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		ip = remote
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	window, exists := l.windows[ip]
	if !exists && len(l.windows) >= 4096 {
		for key, value := range l.windows {
			if now.Sub(value.started) >= time.Minute {
				delete(l.windows, key)
			}
		}
		if len(l.windows) >= 4096 {
			return false
		}
	}
	if now.Sub(window.started) >= time.Minute {
		window = loginWindow{started: now}
	}
	if window.attempts >= 10 {
		return false
	}
	window.attempts++
	l.windows[ip] = window
	return true
}
