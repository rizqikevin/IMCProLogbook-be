package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"machine-logbook/internal/domain"
	"machine-logbook/internal/service"
)

type Options struct {
	Logger               *slog.Logger
	AllowedOrigins       []string
	MaxRequestBytes      int64
	MaxConcurrentUploads int
	RequestTimeout       time.Duration
	Ready                func(context.Context) error
}

type Server struct {
	logbooks     *service.Logbooks
	auth         *service.Auth
	opts         Options
	uploads      chan struct{}
	loginLimiter *loginLimiter
}

func New(logbooks *service.Logbooks, auth *service.Auth, opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.MaxConcurrentUploads < 1 {
		opts.MaxConcurrentUploads = 4
	}
	if opts.MaxRequestBytes < 1 {
		opts.MaxRequestBytes = 100 << 20
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 2 * time.Minute
	}
	s := &Server{logbooks: logbooks, auth: auth, opts: opts, uploads: make(chan struct{}, opts.MaxConcurrentUploads), loginLimiter: newLoginLimiter()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", s.ready)
	mux.HandleFunc("GET /docs", docsUI)
	mux.HandleFunc("GET /docs/openapi.yaml", docsSpec)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.Handle("POST /api/v1/auth/logout", s.authenticated(http.HandlerFunc(s.logout)))
	mux.Handle("GET /api/v1/auth/me", s.authenticated(http.HandlerFunc(s.me)))
	mux.Handle("GET /api/v1/machines", s.authenticated(http.HandlerFunc(s.machines)))
	mux.Handle("GET /api/v1/shifts", s.authenticated(http.HandlerFunc(s.shifts)))
	mux.Handle("GET /api/v1/logbooks", s.authenticated(http.HandlerFunc(s.list)))
	mux.Handle("POST /api/v1/logbooks", s.authenticated(s.upload(http.HandlerFunc(s.create))))
	mux.Handle("GET /api/v1/logbooks/{id}", s.authenticated(http.HandlerFunc(s.get)))
	mux.Handle("POST /api/v1/logbooks/{id}/photos", s.authenticated(s.upload(http.HandlerFunc(s.appendPhotos))))
	mux.Handle("GET /api/v1/logbooks/{id}/photos/{photoID}", s.authenticated(http.HandlerFunc(s.photo)))
	mux.Handle("DELETE /api/v1/logbooks/{id}", s.authenticated(s.admin(http.HandlerFunc(s.delete))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		problem(w, r, http.StatusNotFound, "not_found", "endpoint not found")
	})
	return s.middleware(mux)
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if s.opts.Ready != nil {
		if err := s.opts.Ready(ctx); err != nil {
			problem(w, r, http.StatusServiceUnavailable, "not_ready", "service is not ready")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.loginLimiter.allow(r.RemoteAddr, time.Now()) {
		w.Header().Set("Retry-After", "60")
		problem(w, r, http.StatusTooManyRequests, "rate_limited", "too many login attempts, retry later")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		problem(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "use application/json")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		s.decodeError(w, r, err)
		return
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		problem(w, r, http.StatusBadRequest, "invalid_json", "expected one JSON object")
		return
	}
	result, err := s.auth.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), bearer(r)); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}
func (s *Server) machines(w http.ResponseWriter, r *http.Request) {
	items, err := s.logbooks.Machines(r.Context())
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) shifts(w http.ResponseWriter, r *http.Request) {
	items, err := s.logbooks.Shifts(r.Context())
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	book, err := s.logbooks.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		problem(w, r, 400, "validation_error", "invalid query string")
		return
	}
	allowed := map[string]bool{"machine_id": true, "shift_id": true, "date_from": true, "date_to": true, "cursor": true, "limit": true}
	for key, values := range q {
		if !allowed[key] || len(values) != 1 || values[0] == "" {
			problem(w, r, 400, "validation_error", "invalid or repeated query parameter: "+key)
			return
		}
	}
	f := domain.Filter{DateFrom: q.Get("date_from"), DateTo: q.Get("date_to"), Limit: 20}
	for key, target := range map[string]*int64{"machine_id": &f.MachineID, "shift_id": &f.ShiftID} {
		if raw := q.Get(key); raw != "" {
			n, e := strconv.ParseInt(raw, 10, 64)
			if e != nil || n < 1 {
				problem(w, r, 400, "validation_error", key+" must be a positive integer")
				return
			}
			*target = n
		}
	}
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			problem(w, r, 400, "validation_error", "limit must be between 1 and 100")
			return
		}
		f.Limit = n
	}
	f.Cursor, err = service.ParseCursor(q.Get("cursor"))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	page, err := s.logbooks.List(r.Context(), f)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	uploads, err := s.multipart(w, r, true)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	machineID, e1 := strconv.ParseInt(r.MultipartForm.Value["machine_id"][0], 10, 64)
	shiftID, e2 := strconv.ParseInt(r.MultipartForm.Value["shift_id"][0], 10, 64)
	if e1 != nil || e2 != nil {
		problem(w, r, 400, "validation_error", "machine_id and shift_id must be positive integers")
		return
	}
	book, err := s.logbooks.Create(r.Context(), currentUser(r), machineID, r.MultipartForm.Value["log_date"][0], shiftID, uploads)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/logbooks/"+book.ID)
	writeJSON(w, http.StatusCreated, book)
}

func (s *Server) appendPhotos(w http.ResponseWriter, r *http.Request) {
	uploads, err := s.multipart(w, r, false)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	defer r.MultipartForm.RemoveAll()
	book, err := s.logbooks.Append(r.Context(), currentUser(r), r.PathValue("id"), uploads)
	if err != nil {
		s.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (s *Server) multipart(w http.ResponseWriter, r *http.Request, create bool) ([]service.Upload, error) {
	r.Body = http.MaxBytesReader(w, r.Body, s.opts.MaxRequestBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, err
		}
		return nil, &domain.ValidationError{Message: "expected a valid multipart/form-data body"}
	}
	invalid := func(message string) ([]service.Upload, error) {
		_ = r.MultipartForm.RemoveAll()
		return nil, &domain.ValidationError{Message: message}
	}
	allowed := map[string]bool{}
	if create {
		allowed = map[string]bool{"machine_id": true, "log_date": true, "shift_id": true}
	}
	for key, values := range r.MultipartForm.Value {
		if !allowed[key] || len(values) != 1 {
			return invalid("unknown or repeated form field: " + key)
		}
	}
	for key := range allowed {
		if len(r.MultipartForm.Value[key]) != 1 {
			return invalid("required form field: " + key)
		}
	}
	for key := range r.MultipartForm.File {
		if key != "photos" {
			return invalid("file field must be named photos")
		}
	}
	files := r.MultipartForm.File["photos"]
	result := make([]service.Upload, 0, len(files))
	for _, file := range files {
		result = append(result, service.Upload{Size: file.Size, Open: func() (io.ReadCloser, error) { return file.Open() }})
	}
	return result, nil
}

func (s *Server) photo(w http.ResponseWriter, r *http.Request) {
	photo, body, err := s.logbooks.OpenPhoto(r.Context(), r.PathValue("id"), r.PathValue("photoID"))
	if err != nil {
		s.failure(w, r, err)
		return
	}
	defer body.Close()
	w.Header().Set("Content-Type", photo.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(photo.Size, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": photo.ID + extension(photo.ContentType)}))
	w.Header().Set("ETag", `"`+photo.SHA256+`"`)
	if r.Method == http.MethodHead {
		return
	}
	if _, err = io.Copy(w, body); err != nil {
		s.opts.Logger.Error("stream photo failed", "request_id", requestID(r), "error", err)
	}
}
func extension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}
	return ""
}
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	if err := s.logbooks.Delete(r.Context(), r.PathValue("id")); err != nil {
		s.failure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func problem(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "request_id": requestID(r)}})
}
func (s *Server) decodeError(w http.ResponseWriter, r *http.Request, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		s.failure(w, r, err)
		return
	}
	problem(w, r, 400, "invalid_json", "invalid JSON body")
}
func (s *Server) failure(w http.ResponseWriter, r *http.Request, err error) {
	var validation *domain.ValidationError
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		problem(w, r, 413, "payload_too_large", "request body exceeds the upload limit")
	case errors.As(err, &validation):
		problem(w, r, 400, "validation_error", validation.Message)
	case errors.Is(err, domain.ErrNotFound):
		problem(w, r, 404, "not_found", "resource not found")
	case errors.Is(err, domain.ErrConflict):
		problem(w, r, 409, "conflict", domain.ErrConflict.Error())
	case errors.Is(err, domain.ErrUserExists):
		problem(w, r, 409, "conflict", domain.ErrUserExists.Error())
	case errors.Is(err, domain.ErrInvalidReference):
		problem(w, r, 400, "validation_error", domain.ErrInvalidReference.Error())
	case errors.Is(err, domain.ErrTooManyPhotos):
		problem(w, r, 409, "photo_limit", domain.ErrTooManyPhotos.Error())
	case errors.Is(err, domain.ErrUploadExpired):
		problem(w, r, 409, "upload_expired", "upload expired, please retry")
	case errors.Is(err, domain.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		problem(w, r, 401, "unauthorized", "invalid credentials or expired session")
	case errors.Is(err, context.DeadlineExceeded):
		problem(w, r, 504, "timeout", "request timed out")
	case errors.Is(err, context.Canceled):
		problem(w, r, 408, "request_canceled", "request canceled")
	default:
		s.opts.Logger.Error("request failed", "request_id", requestID(r), "error", err)
		problem(w, r, 500, "internal_error", "an internal error occurred")
	}
}

func bearer(r *http.Request) string {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}
