package config

import (
	"os"
	"testing"

	"github.com/SherClockHolmes/webpush-go"
)

func TestVAPIDConfigValidation(t *testing.T) {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil { t.Fatal(err) }
	base := map[string]string{"DATABASE_URL": "postgres://db", "VAPID_PUBLIC_KEY": publicKey, "VAPID_PRIVATE_KEY": privateKey, "VAPID_SUBJECT": "mailto:admin@example.com"}
	for key, value := range base { t.Setenv(key, value) }
	if _, err := Load(); err != nil { t.Fatalf("valid VAPID config: %v", err) }
	for _, key := range []string{"VAPID_PRIVATE_KEY", "VAPID_SUBJECT"} {
		old := os.Getenv(key)
		t.Setenv(key, "")
		if _, err := Load(); err == nil { t.Fatalf("expected all-or-none error after clearing %s", key) }
		t.Setenv(key, old)
	}
	t.Setenv("VAPID_SUBJECT", "https://example.com/push")
	if _, err := Load(); err != nil { t.Fatalf("valid HTTPS subject: %v", err) }
	t.Setenv("VAPID_SUBJECT", "http://example.com")
	if _, err := Load(); err == nil { t.Fatal("expected invalid subject error") }
}
