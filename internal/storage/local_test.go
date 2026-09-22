package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"machine-logbook/internal/domain"
)

const testKey = "logbooks/550e8400-e29b-41d4-a716-446655440000/550e8400-e29b-41d4-a716-446655440001.jpg"

func newTestLocal(t *testing.T) (*Local, string) {
	t.Helper()
	directory := t.TempDir()
	store, err := NewLocal(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, directory
}

func TestLocalRoundTrip(t *testing.T) {
	store, directory := newTestLocal(t)
	ctx := context.Background()
	if err := store.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(ctx, testKey, strings.NewReader("image bytes"), 11, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	file, err := store.Open(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil || string(data) != "image bytes" {
		t.Fatalf("read = %q, %v", data, err)
	}
	info, err := os.Stat(filepath.Join(directory, testKey))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("object mode = %v, want 0600", info.Mode())
	}
	for range 2 {
		if err := store.Delete(ctx, testKey); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.Open(ctx, testKey); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing object error = %v", err)
	}
}

func TestLocalRejectsUnsafeKeys(t *testing.T) {
	store, _ := newTestLocal(t)
	ctx := context.Background()
	for _, key := range []string{"", "../secret", "/tmp/secret", testKey + "/..", strings.Replace(testKey, "/", `\`, -1), strings.Replace(testKey, ".jpg", ".html", 1), "logbooks/../" + testKey, testKey + "\x00"} {
		t.Run(key, func(t *testing.T) {
			if err := store.Put(ctx, key, strings.NewReader("x"), 1, "image/jpeg"); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Put error = %v", err)
			}
			if _, err := store.Open(ctx, key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Open error = %v", err)
			}
			if err := store.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("Delete error = %v", err)
			}
		})
	}
}

func TestLocalSymlinkCannotEscapeRoot(t *testing.T) {
	store, directory := newTestLocal(t)
	outside := t.TempDir()
	bookDirectory := strings.Split(testKey, "/")[1]
	if err := os.MkdirAll(filepath.Join(outside, bookDirectory), 0o700); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, bookDirectory, filepath.Base(testKey))
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "logbooks")); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, testKey, strings.NewReader("changed"), 7, "image/jpeg"); err == nil {
		t.Fatal("Put followed symlink outside root")
	}
	if file, err := store.Open(ctx, testKey); err == nil {
		_ = file.Close()
		t.Fatal("Open followed symlink outside root")
	}
	if err := store.Delete(ctx, testKey); err == nil {
		t.Fatal("Delete followed symlink outside root")
	}
	data, err := os.ReadFile(outsideFile)
	if err != nil || string(data) != "secret" {
		t.Fatalf("outside file changed: %q, %v", data, err)
	}
}

func TestLocalFailedPutPreservesExistingObject(t *testing.T) {
	store, directory := newTestLocal(t)
	ctx := context.Background()
	if err := store.Put(ctx, testKey, strings.NewReader("original"), 8, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	readErr := errors.New("camera stream interrupted")
	for _, test := range []struct {
		name string
		body io.Reader
		size int64
		want error
	}{
		{"short", strings.NewReader("short"), 10, ErrSizeMismatch},
		{"long", strings.NewReader("too long"), 3, ErrSizeMismatch},
		{"zero", strings.NewReader(""), 0, ErrSizeMismatch},
		{"read error", io.MultiReader(strings.NewReader("partial"), failingReader{err: readErr}), 20, readErr},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := store.Put(ctx, testKey, test.body, test.size, "image/jpeg")
			if !errors.Is(err, test.want) {
				t.Fatalf("Put error = %v, want %v", err, test.want)
			}
			data, err := os.ReadFile(filepath.Join(directory, testKey))
			if err != nil || string(data) != "original" {
				t.Fatalf("existing object = %q, %v", data, err)
			}
			assertNoTemporaryObjects(t, directory)
		})
	}
}

func TestLocalCancellationCleansIncompleteObject(t *testing.T) {
	store, directory := newTestLocal(t)
	ctx, cancel := context.WithCancel(context.Background())
	err := store.Put(ctx, testKey, cancelingReader{cancel: cancel}, 8, "image/jpeg")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Put error = %v", err)
	}
	if _, err := store.Open(context.Background(), testKey); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("incomplete object exists: %v", err)
	}
	assertNoTemporaryObjects(t, directory)
	if _, err := store.Open(ctx, testKey); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Open = %v", err)
	}
	if err := store.Delete(ctx, testKey); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Delete = %v", err)
	}
	if err := store.Check(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Check = %v", err)
	}
}

func TestLocalOpenReadHonorsCancellation(t *testing.T) {
	store, _ := newTestLocal(t)
	if err := store.Put(context.Background(), testKey, strings.NewReader("image"), 5, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	file, err := store.Open(ctx, testKey)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	cancel()
	if _, err := io.ReadAll(file); !errors.Is(err, context.Canceled) {
		t.Fatalf("Read after cancellation = %v", err)
	}
}

func assertNoTemporaryObjects(t *testing.T, directory string) {
	t.Helper()
	if err := filepath.WalkDir(directory, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".upload-") || strings.HasPrefix(entry.Name(), ".health-") {
			t.Errorf("temporary file remains: %s", name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type cancelingReader struct{ cancel context.CancelFunc }

func (r cancelingReader) Read(p []byte) (int, error) {
	r.cancel()
	return copy(p, "partial"), nil
}
