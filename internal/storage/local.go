package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path"

	"machine-logbook/internal/domain"
)

type Local struct {
	root *os.Root
}

func NewLocal(directory string) (*Local, error) {
	if directory == "" {
		return nil, errors.New("local storage directory is required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return nil, fmt.Errorf("open storage directory: %w", err)
	}
	return &Local{root: root}, nil
}

func (s *Local) Close() error { return s.root.Close() }

func (s *Local) Put(ctx context.Context, key string, body io.Reader, size int64, _ string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory := path.Dir(key)
	if err := s.root.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create object directory: %w", err)
	}
	temporary := path.Join(directory, ".upload-"+rand.Text())
	file, err := s.root.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create temporary object: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = s.root.Remove(temporary)
	}()
	if err := copyExact(ctx, file, body, size); err != nil {
		return fmt.Errorf("write object: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync object: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close object: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.root.Rename(temporary, key); err != nil {
		return fmt.Errorf("publish object: %w", err)
	}
	parent, err := s.root.Open(directory)
	if err != nil {
		return fmt.Errorf("open object directory: %w", err)
	}
	defer parent.Close()
	if err := parent.Sync(); err != nil {
		return fmt.Errorf("sync object directory: %w", err)
	}
	return nil
}

func (s *Local) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := validateKey(key); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := s.root.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("open object: %w", err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		if err != nil {
			return nil, fmt.Errorf("stat object: %w", err)
		}
		return nil, errors.New("object is not a regular file")
	}
	return &contextReadCloser{contextReader: contextReader{ctx: ctx, reader: file}, closer: file}, nil
}

func (s *Local) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.root.Remove(key); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

func (s *Local) Check(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := ".health-" + rand.Text()
	file, err := s.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("check storage write access: %w", err)
	}
	closeErr := file.Close()
	removeErr := s.root.Remove(name)
	if closeErr != nil {
		return fmt.Errorf("close storage check: %w", closeErr)
	}
	if removeErr != nil {
		return fmt.Errorf("remove storage check: %w", removeErr)
	}
	return ctx.Err()
}
