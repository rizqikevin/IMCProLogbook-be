package config

import (
	"os"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"
)

func TestVAPIDConfigValidation(t *testing.T) {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]string{"DATABASE_URL": "postgres://db", "VAPID_PUBLIC_KEY": publicKey, "VAPID_PRIVATE_KEY": privateKey, "VAPID_SUBJECT": "mailto:admin@example.com"}
	for key, value := range base {
		t.Setenv(key, value)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("valid VAPID config: %v", err)
	}
	for _, key := range []string{"VAPID_PRIVATE_KEY", "VAPID_SUBJECT"} {
		old := os.Getenv(key)
		t.Setenv(key, "")
		if _, err := Load(); err == nil {
			t.Fatalf("expected all-or-none error after clearing %s", key)
		}
		t.Setenv(key, old)
	}
	t.Setenv("VAPID_SUBJECT", "https://example.com/push")
	if _, err := Load(); err != nil {
		t.Fatalf("valid HTTPS subject: %v", err)
	}
	t.Setenv("VAPID_SUBJECT", "http://example.com")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid subject error")
	}
}

func TestPersistentSessionConfig(t *testing.T) {
	for _, key := range []string{"VAPID_PUBLIC_KEY", "VAPID_PRIVATE_KEY", "VAPID_SUBJECT", "SESSION_TTL", "SESSION_COOKIE_SECURE", "CORS_ALLOWED_ORIGINS"} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("APP_ENV", "production")
	c, err := Load()
	if err != nil || c.SessionTTL != 720*time.Hour || !c.SessionCookieSecure {
		t.Fatalf("production defaults: %+v %v", c, err)
	}
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("production must reject insecure cookies")
	}
	t.Setenv("APP_ENV", "development")
	c, err = Load()
	if err != nil || c.SessionCookieSecure {
		t.Fatalf("development HTTP: %+v %v", c, err)
	}
	t.Setenv("SESSION_COOKIE_SECURE", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("invalid cookie setting accepted")
	}
	t.Setenv("SESSION_COOKIE_SECURE", "false")
	t.Setenv("SESSION_TTL", "721h")
	if _, err := Load(); err == nil {
		t.Fatal("TTL over 30 days accepted")
	}
}
