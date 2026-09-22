package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"machine-logbook/internal/domain"
)

type AuthRepository interface {
	UserByUsername(context.Context, string) (domain.User, error)
	CreateUser(context.Context, domain.User) error
	SetPassword(context.Context, string, string) error
	DisableUser(context.Context, string) error
	CreateSession(context.Context, domain.Session) error
	Session(context.Context, string) (domain.Session, error)
	DeleteSession(context.Context, string) error
	PurgeSessions(context.Context) error
}

type Auth struct {
	repo       AuthRepository
	sessionTTL time.Duration
}

type LoginResult struct {
	AccessToken string      `json:"access_token"`
	TokenType   string      `json:"token_type"`
	ExpiresAt   time.Time   `json:"expires_at"`
	User        domain.User `json:"user"`
}

func NewAuth(repo AuthRepository, sessionTTL time.Duration) *Auth {
	return &Auth{repo: repo, sessionTTL: sessionTTL}
}

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (a *Auth) Login(ctx context.Context, username, password string) (LoginResult, error) {
	username, valid := normalizeUsername(username)
	if !valid {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		return LoginResult{}, domain.ErrUnauthorized
	}
	user, err := a.repo.UserByUsername(ctx, username)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
		if errors.Is(err, domain.ErrNotFound) {
			return LoginResult{}, domain.ErrUnauthorized
		}
		return LoginResult{}, fmt.Errorf("find login user: %w", err)
	}
	passwordErr := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if passwordErr != nil || !user.Active {
		return LoginResult{}, domain.ErrUnauthorized
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random[:])
	expiresAt := time.Now().UTC().Add(a.sessionTTL)
	if err := a.repo.CreateSession(ctx, domain.Session{TokenHash: tokenHash(token), User: user, ExpiresAt: expiresAt}); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}
	user.PasswordHash = ""
	return LoginResult{AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
}

func (a *Auth) Authenticate(ctx context.Context, token string) (domain.User, error) {
	if !validToken(token) {
		return domain.User{}, domain.ErrUnauthorized
	}
	session, err := a.repo.Session(ctx, tokenHash(token))
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrUnauthorized) {
		return domain.User{}, domain.ErrUnauthorized
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("lookup session: %w", err)
	}
	if !session.User.Active || !session.ExpiresAt.After(time.Now()) {
		return domain.User{}, domain.ErrUnauthorized
	}
	session.User.PasswordHash = ""
	return session.User, nil
}

func (a *Auth) Logout(ctx context.Context, token string) error {
	if !validToken(token) {
		return domain.ErrUnauthorized
	}
	if err := a.repo.DeleteSession(ctx, tokenHash(token)); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (a *Auth) Provision(ctx context.Context, username, name, password, role string) error {
	username, valid := normalizeUsername(username)
	if !valid {
		return invalid("username must contain 3 to 64 ASCII letters, digits, dots, underscores, or hyphens")
	}
	name = strings.TrimSpace(name)
	if !utf8.ValidString(name) || utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 120 {
		return invalid("name must contain 1 to 120 characters")
	}
	if role != "operator" && role != "admin" {
		return invalid("role must be operator or admin")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return a.repo.CreateUser(ctx, domain.User{ID: uuid.NewString(), Username: username, Name: name, Role: role, Active: true, PasswordHash: hash})
}

func (a *Auth) ResetPassword(ctx context.Context, username, password string) error {
	username, valid := normalizeUsername(username)
	if !valid {
		return invalid("invalid username")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	return a.repo.SetPassword(ctx, username, hash)
}

func (a *Auth) Disable(ctx context.Context, username string) error {
	username, valid := normalizeUsername(username)
	if !valid {
		return invalid("invalid username")
	}
	return a.repo.DisableUser(ctx, username)
}

func (a *Auth) Cleanup(ctx context.Context) error {
	return a.repo.PurgeSessions(ctx)
}

func normalizeUsername(username string) (string, bool) {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 64 {
		return "", false
	}
	for _, c := range username {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '.' && c != '_' && c != '-' {
			return "", false
		}
	}
	return strings.ToLower(username), true
}

func hashPassword(password string) (string, error) {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 12 || len(password) > 72 {
		return "", invalid("password must contain at least 12 characters and at most 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func validToken(token string) bool {
	if len(token) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(decoded) == 32
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func invalid(message string) error { return &domain.ValidationError{Message: message} }
