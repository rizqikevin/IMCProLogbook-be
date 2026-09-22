package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
)

type Store interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Check(ctx context.Context) error
}

var (
	ErrInvalidKey   = errors.New("invalid storage key")
	ErrSizeMismatch = errors.New("object size does not match declared size")
	keyPattern      = regexp.MustCompile(`^logbooks/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png|webp)$`)
)

func validateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return ErrInvalidKey
	}
	return nil
}

func copyExact(ctx context.Context, dst io.Writer, src io.Reader, size int64) error {
	if size <= 0 || size == math.MaxInt64 {
		return ErrSizeMismatch
	}
	n, err := io.Copy(dst, io.LimitReader(contextReader{ctx: ctx, reader: src}, size+1))
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("%w: expected %d bytes, read %d", ErrSizeMismatch, size, n)
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

type contextReadCloser struct {
	contextReader
	closer io.Closer
}

func (r *contextReadCloser) Close() error { return r.closer.Close() }
