package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"machine-logbook/internal/domain"
)

type authRepoStub struct {
	AuthRepository
	find          func(context.Context, string) (domain.User, error)
	createUser    func(context.Context, domain.User) error
	createSession func(context.Context, domain.Session) error
	session       func(context.Context, string) (domain.Session, error)
	deleteSession func(context.Context, string) error
	setPassword   func(context.Context, string, string) error
	disableUser   func(context.Context, string) error
}

func (r *authRepoStub) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	return r.find(ctx, username)
}

func (r *authRepoStub) CreateUser(ctx context.Context, user domain.User) error {
	return r.createUser(ctx, user)
}

func (r *authRepoStub) CreateSession(ctx context.Context, session domain.Session) error {
	return r.createSession(ctx, session)
}

func (r *authRepoStub) Session(ctx context.Context, hash string) (domain.Session, error) {
	return r.session(ctx, hash)
}

func (r *authRepoStub) DeleteSession(ctx context.Context, hash string) error {
	return r.deleteSession(ctx, hash)
}

func (r *authRepoStub) SetPassword(ctx context.Context, username, hash string) error {
	return r.setPassword(ctx, username, hash)
}

func (r *authRepoStub) DisableUser(ctx context.Context, username string) error {
	return r.disableUser(ctx, username)
}

func loginUser(t *testing.T) domain.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return domain.User{ID: testUserID, Username: "operator.one", Name: "Operator One", Role: "operator", Active: true, PasswordHash: string(hash)}
}

func TestLoginCreatesHashedOpaqueSession(t *testing.T) {
	user := loginUser(t)
	var saved domain.Session
	repo := &authRepoStub{
		find: func(_ context.Context, username string) (domain.User, error) {
			if username != "operator.one" {
				t.Fatalf("username not normalized: %q", username)
			}
			return user, nil
		},
		createSession: func(_ context.Context, session domain.Session) error {
			saved = session
			return nil
		},
	}
	before := time.Now()
	result, err := NewAuth(repo, 2*time.Hour).Login(context.Background(), "  OPERATOR.One  ", "correct password")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(result.AccessToken)
	if err != nil || len(decoded) != 32 || result.TokenType != "Bearer" {
		t.Fatalf("invalid opaque token: %+v, err %v", result, err)
	}
	digest := sha256.Sum256([]byte(result.AccessToken))
	if saved.TokenHash != hex.EncodeToString(digest[:]) || saved.TokenHash == result.AccessToken {
		t.Fatal("repository must receive only the session token's SHA-256 digest")
	}
	if saved.User.PasswordHash != user.PasswordHash {
		t.Fatal("session creation lost the password hash needed to detect concurrent password reset")
	}
	if result.User.PasswordHash != "" || result.User.ID != user.ID {
		t.Fatal("login response exposes a password hash or wrong user")
	}
	if saved.ExpiresAt != result.ExpiresAt || result.ExpiresAt.Before(before.Add(2*time.Hour)) || result.ExpiresAt.After(time.Now().Add(2*time.Hour)) {
		t.Fatalf("incorrect expiry: %v", result.ExpiresAt)
	}
	second, err := NewAuth(repo, 2*time.Hour).Login(context.Background(), "operator.one", "correct password")
	if err != nil || second.AccessToken == result.AccessToken {
		t.Fatal("each login must get a fresh random token")
	}
}

func TestLoginFailuresDoNotCreateSession(t *testing.T) {
	user := loginUser(t)
	cases := []struct {
		name     string
		username string
		password string
		active   bool
		missing  bool
	}{
		{name: "invalid username", username: "x", password: "correct password", active: true},
		{name: "missing operator", username: "operator.one", password: "correct password", active: true, missing: true},
		{name: "wrong password", username: "operator.one", password: "wrong password", active: true},
		{name: "inactive operator", username: "operator.one", password: "correct password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &authRepoStub{find: func(context.Context, string) (domain.User, error) {
				if tc.missing {
					return domain.User{}, domain.ErrNotFound
				}
				copy := user
				copy.Active = tc.active
				return copy, nil
			}}
			result, err := NewAuth(repo, time.Hour).Login(context.Background(), tc.username, tc.password)
			if !errors.Is(err, domain.ErrUnauthorized) || result.AccessToken != "" {
				t.Fatalf("expected uniform unauthorized response, result=%+v err=%v", result, err)
			}
		})
	}
}

func TestLoginConcurrentRevocationDoesNotIssueToken(t *testing.T) {
	user := loginUser(t)
	repo := &authRepoStub{
		find:          func(context.Context, string) (domain.User, error) { return user, nil },
		createSession: func(context.Context, domain.Session) error { return domain.ErrUnauthorized },
	}
	result, err := NewAuth(repo, time.Hour).Login(context.Background(), "operator.one", "correct password")
	if !errors.Is(err, domain.ErrUnauthorized) || result.AccessToken != "" {
		t.Fatalf("issued token after concurrent reset/disable: result=%+v err=%v", result, err)
	}
}

func TestAuthenticateAndLogoutUseOnlyTokenHash(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytes32())
	digest := sha256.Sum256([]byte(token))
	wantHash := hex.EncodeToString(digest[:])
	user := loginUser(t)
	revoked := false
	repo := &authRepoStub{
		session: func(_ context.Context, hash string) (domain.Session, error) {
			if hash != wantHash {
				t.Fatalf("session lookup must use token hash: %q", hash)
			}
			if revoked {
				return domain.Session{}, domain.ErrUnauthorized
			}
			return domain.Session{User: user, ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		deleteSession: func(_ context.Context, hash string) error {
			if hash != wantHash {
				t.Fatalf("logout must use token hash: %q", hash)
			}
			revoked = true
			return nil
		},
	}
	auth := NewAuth(repo, time.Hour)
	authenticated, err := auth.Authenticate(context.Background(), token)
	if err != nil || authenticated.ID != user.ID || authenticated.PasswordHash != "" {
		t.Fatalf("authenticated=%+v err=%v", authenticated, err)
	}
	if err := auth.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("revoked session accepted: %v", err)
	}
}

func TestAuthenticateRejectsExpiredDisabledAndMissingSessions(t *testing.T) {
	token := base64.RawURLEncoding.EncodeToString(bytes32())
	for _, condition := range []string{"expired", "disabled", "missing"} {
		t.Run(condition, func(t *testing.T) {
			session := domain.Session{User: domain.User{Active: true}, ExpiresAt: time.Now().Add(time.Hour)}
			if condition == "expired" {
				session.ExpiresAt = time.Now().Add(-time.Second)
			}
			if condition == "disabled" {
				session.User.Active = false
			}
			repo := &authRepoStub{session: func(context.Context, string) (domain.Session, error) {
				if condition == "missing" {
					return domain.Session{}, domain.ErrNotFound
				}
				return session, nil
			}}
			if _, err := NewAuth(repo, time.Hour).Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatalf("accepted %s session: %v", condition, err)
			}
		})
	}
}

func TestMalformedTokensAreRejectedBeforeRepositoryLookup(t *testing.T) {
	auth := NewAuth(nil, time.Hour)
	for _, token := range []string{"", "short", strings.Repeat("!", 43), strings.Repeat("A", 42) + "B", strings.Repeat("A", 44)} {
		if _, err := auth.Authenticate(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("accepted malformed token %q", token)
		}
		if err := auth.Logout(context.Background(), token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("logout accepted malformed token %q", token)
		}
	}
}

func TestProvisionHashesPasswordAndNormalizesIdentity(t *testing.T) {
	var saved domain.User
	repo := &authRepoStub{createUser: func(_ context.Context, user domain.User) error { saved = user; return nil }}
	if err := NewAuth(repo, time.Hour).Provision(context.Background(), " Operator.One ", " Operator One ", "correct password", "operator"); err != nil {
		t.Fatal(err)
	}
	if !validUUID(saved.ID) || saved.Username != "operator.one" || saved.Name != "Operator One" || !saved.Active || saved.Role != "operator" {
		t.Fatalf("unexpected operator: %+v", saved)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(saved.PasswordHash), []byte("correct password")); err != nil {
		t.Fatal(err)
	}
	cost, err := bcrypt.Cost([]byte(saved.PasswordHash))
	if err != nil || cost < bcrypt.DefaultCost {
		t.Fatalf("weak password hash cost %d: %v", cost, err)
	}
}

func TestProvisionRejectsInvalidOperatorAndPasswords(t *testing.T) {
	cases := []struct{ name, username, displayName, password, role string }{
		{"short username", "ab", "Name", "correct password", "operator"},
		{"invalid username", "a/b", "Name", "correct password", "operator"},
		{"unicode ASCII alias", "Kevin", "Name", "correct password", "operator"},
		{"blank display name", "operator", " ", "correct password", "operator"},
		{"invalid role", "operator", "Name", "correct password", "owner"},
		{"short password", "operator", "Name", "12345678901", "operator"},
		{"bcrypt truncation", "operator", "Name", strings.Repeat("a", 73), "operator"},
		{"short unicode password", "operator", "Name", strings.Repeat("é", 11), "operator"},
		{"multibyte byte limit", "operator", "Name", strings.Repeat("界", 25), "operator"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := NewAuth(nil, time.Hour).Provision(context.Background(), tc.username, tc.displayName, tc.password, tc.role)
			var validation *domain.ValidationError
			if !errors.As(err, &validation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestPasswordResetAndDisableAddressNormalizedUsername(t *testing.T) {
	var resetUsername, newHash, disabledUsername string
	repo := &authRepoStub{
		setPassword: func(_ context.Context, username, hash string) error {
			resetUsername, newHash = username, hash
			return nil
		},
		disableUser: func(_ context.Context, username string) error { disabledUsername = username; return nil },
	}
	auth := NewAuth(repo, time.Hour)
	if err := auth.ResetPassword(context.Background(), " OPERATOR.One ", "new secure password"); err != nil {
		t.Fatal(err)
	}
	if err := auth.Disable(context.Background(), " OPERATOR.One "); err != nil {
		t.Fatal(err)
	}
	if resetUsername != "operator.one" || disabledUsername != "operator.one" {
		t.Fatal("operator username not normalized")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("new secure password")); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryFailureIsNotReportedAsBadCredentials(t *testing.T) {
	failure := errors.New("database unavailable")
	repo := &authRepoStub{find: func(context.Context, string) (domain.User, error) { return domain.User{}, failure }}
	_, err := NewAuth(repo, time.Hour).Login(context.Background(), "operator.one", "correct password")
	if !errors.Is(err, failure) || errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("unexpected repository error: %v", err)
	}
}

func bytes32() []byte {
	value := make([]byte, 32)
	for index := range value {
		value[index] = byte(index + 1)
	}
	return value
}
