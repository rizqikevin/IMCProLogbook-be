package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"machine-logbook/internal/httpapi"
	"machine-logbook/internal/repository"
	"machine-logbook/internal/service"
	"machine-logbook/internal/storage"
	"machine-logbook/migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("TEST_DATABASE_URL must point to a test PostgreSQL instance")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer admin.Close()
	schema := "frontend_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		return err
	}
	defer func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	u, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	if err := migrations.Up(ctx, u.String()); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return err
	}
	defer pool.Close()
	directory, err := os.MkdirTemp("", "logbook-frontend-e2e-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	store, err := storage.NewLocal(directory)
	if err != nil {
		return err
	}
	defer store.Close()
	repo := repository.New(pool)
	auth := service.NewAuth(repo, time.Hour)
	for _, role := range []string{"operator", "admin"} {
		if err := auth.Provision(ctx, role, "Pengujian "+role, "frontend-test-only-123", role); err != nil {
			return err
		}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	books := service.NewLogbooks(repo, store, service.Limits{MaxPhotosPerUpload: 20, MaxPhotosPerLogbook: 100, MaxPhotoBytes: 10 << 20, MaxImagePixels: 40_000_000}, logger)
	server := &http.Server{Addr: "127.0.0.1:18081", Handler: httpapi.New(books, auth, httpapi.Options{Logger: logger, Ready: pool.Ping}), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
