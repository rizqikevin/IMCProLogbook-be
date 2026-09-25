package config

import (
	"strings"
	"testing"
)

func baseEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"APP_ENV", "HTTP_ADDR", "DATABASE_URL", "DATABASE_MAX_CONNS", "STORAGE_DRIVER", "LOCAL_STORAGE_PATH", "S3_BUCKET", "AWS_REGION", "S3_ENDPOINT", "S3_PATH_STYLE", "CORS_ALLOWED_ORIGINS", "SESSION_TTL", "REQUEST_TIMEOUT", "SHUTDOWN_TIMEOUT", "MAX_REQUEST_BYTES", "MAX_PHOTO_BYTES", "MAX_IMAGE_PIXELS", "MAX_PHOTOS_PER_UPLOAD", "MAX_PHOTOS_PER_LOGBOOK", "MAX_CONCURRENT_UPLOADS"} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_URL", "postgres://localhost/logbook")
}

func TestLoadDefaults(t *testing.T) {
	baseEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxRequestBytes != 100<<20 || c.MaxPhotoBytes != 10<<20 || c.StorageDriver != "local" || c.MaxConcurrentUploads != 4 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestRejectsUnsafeConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name    string
		env     map[string]string
		message string
	}{
		{"database", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"s3 bucket", map[string]string{"STORAGE_DRIVER": "s3"}, "S3_BUCKET"},
		{"s3 TLS", map[string]string{"APP_ENV": "production", "STORAGE_DRIVER": "s3", "S3_BUCKET": "archive", "S3_ENDPOINT": "http://s3.example.test"}, "requires HTTPS"},
		{"endpoint scheme", map[string]string{"S3_ENDPOINT": "ftp://storage.test"}, "endpoint URL"},
		{"wildcard CORS", map[string]string{"CORS_ALLOWED_ORIGINS": "*"}, "exact http(s) origins"},
		{"CORS path", map[string]string{"CORS_ALLOWED_ORIGINS": "https://app.test/"}, "exact http(s) origins"},
		{"zero concurrency", map[string]string{"MAX_CONCURRENT_UPLOADS": "0"}, "MAX_CONCURRENT_UPLOADS"},
		{"oversized integer", map[string]string{"MAX_PHOTO_BYTES": "999999999999999999999"}, "MAX_PHOTO_BYTES"},
		{"batch count", map[string]string{"MAX_PHOTOS_PER_UPLOAD": "20", "MAX_PHOTOS_PER_LOGBOOK": "10"}, "cannot exceed"},
		{"bytes", map[string]string{"MAX_REQUEST_BYTES": "1048576"}, "cannot exceed"},
		{"long timeout", map[string]string{"REQUEST_TIMEOUT": "48h"}, "REQUEST_TIMEOUT"},
		{"invalid boolean", map[string]string{"S3_PATH_STYLE": "maybe"}, "S3_PATH_STYLE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("got %v, want %q", err, tc.message)
			}
		})
	}
}

func TestProductionLocalStorage(t *testing.T) {
	baseEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("STORAGE_DRIVER", "local")
	t.Setenv("LOCAL_STORAGE_PATH", "/app/data/uploads")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.StorageDriver != "local" || c.LocalStoragePath != "/app/data/uploads" {
		t.Fatal("local storage configuration not preserved")
	}
}
