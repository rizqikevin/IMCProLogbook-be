package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("logbook already exists for this machine, date, and shift")
	ErrUserExists       = errors.New("username already taken")
	ErrInvalidReference = errors.New("machine or shift does not exist or is inactive")
	ErrUnauthorized     = errors.New("invalid credentials or session")
	ErrUploadExpired    = errors.New("upload reservation expired")
	ErrTooManyPhotos    = errors.New("logbook photo limit exceeded")
)

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type Machine struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type Shift struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	Active       bool   `json:"active"`
	PasswordHash string `json:"-"`
}
type Session struct {
	TokenHash string
	User      User
	ExpiresAt time.Time
}
type Photo struct {
	ID          string    `json:"id"`
	PageNumber  int       `json:"page_number"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	StorageKey  string    `json:"-"`
	SHA256      string    `json:"sha256"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	URL         string    `json:"url"`
}
type Logbook struct {
	ID         string    `json:"id"`
	Machine    Machine   `json:"machine"`
	LogDate    string    `json:"log_date"`
	Shift      Shift     `json:"shift"`
	CreatedBy  string    `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	PhotoCount int       `json:"photo_count"`
	Photos     []Photo   `json:"photos,omitempty"`
}
type Cursor struct {
	Date string `json:"date"`
	ID   string `json:"id"`
}
type Filter struct {
	MachineID int64
	ShiftID   int64
	DateFrom  string
	DateTo    string
	Cursor    *Cursor
	Limit     int
}
type Page struct {
	Items      []Logbook `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}
