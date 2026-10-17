package storage

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"machine-logbook/internal/domain"
)

func newTestS3(t *testing.T, handler http.HandlerFunc) (*S3, string) {
	t.Helper()
	return newTestS3WithPrefix(t, handler, "")
}

func newTestS3WithPrefix(t *testing.T, handler http.HandlerFunc, prefix string) (*S3, string) {
	t.Helper()
	directory := t.TempDir()
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(directory, "no-credentials"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(directory, "no-config"))
	t.Setenv("TMPDIR", directory)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	store, err := NewS3(context.Background(), S3Config{Bucket: "logbooks", Region: "us-east-1", Endpoint: server.URL, PathStyle: true, KeyPrefix: prefix})
	if err != nil {
		t.Fatal(err)
	}
	return store, directory
}

func TestS3RoundTripAndPrivateUpload(t *testing.T) {
	for _, prefix := range []string{"", "archieve", "archieve/", "photos/archieve.v1/"} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			testS3RoundTrip(t, prefix)
		})
	}
}

func testS3RoundTrip(t *testing.T, prefix string) {
	t.Helper()
	objectPrefix := prefix
	if objectPrefix != "" && !strings.HasSuffix(objectPrefix, "/") {
		objectPrefix += "/"
	}
	var object []byte
	store, directory := newTestS3WithPrefix(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Error("request is not signed")
		}
		if r.Method == http.MethodHead {
			if r.URL.Path != "/logbooks" {
				t.Errorf("HeadBucket path = %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path != "/logbooks/"+objectPrefix+testKey {
			t.Errorf("object path = %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodPut:
			if r.Header.Get("x-amz-acl") != "" {
				t.Error("upload unexpectedly sets an ACL")
			}
			if r.Header.Get("Content-Type") != "image/jpeg" || r.ContentLength != 5 {
				t.Errorf("upload metadata = %q, %d", r.Header.Get("Content-Type"), r.ContentLength)
			}
			var err error
			object, err = io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			w.Header().Set("ETag", `"test-etag"`)
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			if object == nil {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(w, `<Error><Code>NoSuchKey</Code><Message>Missing</Message></Error>`)
				return
			}
			_, _ = w.Write(object)
		case http.MethodDelete:
			object = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}, prefix)
	ctx := context.Background()
	if err := store.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(ctx, testKey); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing object = %v", err)
	}
	if err := store.Put(ctx, testKey, strings.NewReader("image"), 5, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	file, err := store.Open(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(data) != "image" {
		t.Fatalf("object = %q, %v", data, err)
	}
	for range 2 {
		if err := store.Delete(ctx, testKey); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Open(ctx, testKey); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted object = %v", err)
	}
	assertS3BufferRemoved(t, directory)
}

func TestS3RejectsMalformedPrefix(t *testing.T) {
	for _, prefix := range []string{"/", "/archieve", "../archieve", "archieve/..", "./archieve", "archieve/./photos", "archieve//photos", "archieve//", `C:\photos`, `archieve\photos`, " archieve", "archieve ", "archieve\n", "archieve/%2e%2e", "archieve?query", "archieve#fragment"} {
		t.Run(prefix, func(t *testing.T) {
			store, err := NewS3(context.Background(), S3Config{Bucket: "logbooks", Region: "auto", KeyPrefix: prefix})
			if err == nil || store != nil {
				t.Fatalf("NewS3 prefix %q = %v, %v", prefix, store, err)
			}
		})
	}
}

func TestS3RejectsInvalidObjectBeforeSending(t *testing.T) {
	for _, prefix := range []string{"", "archieve/"} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			testS3RejectsInvalidObject(t, prefix)
		})
	}
}

func testS3RejectsInvalidObject(t *testing.T, prefix string) {
	t.Helper()
	var requests atomic.Int32
	store, directory := newTestS3WithPrefix(t, func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}, prefix)
	ctx := context.Background()
	for _, size := range []int64{0, 2, 9} {
		if err := store.Put(ctx, testKey, strings.NewReader("image"), size, "image/jpeg"); !errors.Is(err, ErrSizeMismatch) {
			t.Fatalf("Put size %d = %v", size, err)
		}
	}
	if err := store.Put(ctx, "../escape", strings.NewReader("image"), 5, "image/jpeg"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key Put = %v", err)
	}
	if _, err := store.Open(ctx, "../escape"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key Open = %v", err)
	}
	if err := store.Delete(ctx, "../escape"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("invalid key Delete = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("sent %d requests for invalid data", requests.Load())
	}
	assertS3BufferRemoved(t, directory)
}

func TestS3DeadlineAndBufferCleanup(t *testing.T) {
	release := make(chan struct{})
	store, directory := newTestS3(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := store.Put(ctx, testKey, strings.NewReader("image"), 5, "image/jpeg")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Put error = %v", err)
	}
	assertS3BufferRemoved(t, directory)
}

func TestS3PermissionErrorIsNotMissing(t *testing.T) {
	for _, prefix := range []string{"", "archieve/"} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			testS3PermissionError(t, prefix)
		})
	}
}

func testS3PermissionError(t *testing.T, prefix string) {
	t.Helper()
	store, _ := newTestS3WithPrefix(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>Denied</Message></Error>`)
	}, prefix)
	if _, err := store.Open(context.Background(), testKey); err == nil || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("permission error = %v", err)
	}
	if err := store.Check(context.Background()); err == nil {
		t.Fatal("Check ignored access denied")
	}
	if err := store.Delete(context.Background(), testKey); err == nil {
		t.Fatal("Delete ignored access denied")
	}
}

func assertS3BufferRemoved(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "machine-logbook-s3-") {
			t.Errorf("S3 buffer remains: %s", entry.Name())
		}
	}
}
