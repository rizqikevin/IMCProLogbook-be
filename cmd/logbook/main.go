package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"
	"machine-logbook/internal/config"
	"machine-logbook/internal/httpapi"
	"machine-logbook/internal/repository"
	"machine-logbook/internal/service"
	"machine-logbook/internal/storage"
	"machine-logbook/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if command == "help" || command == "--help" || command == "-h" {
		fmt.Println("Usage: logbook [serve | migrate up|down | user create|reset-password|disable]")
		return nil
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if command == "migrate" {
		if len(os.Args) != 3 {
			return errors.New("usage: logbook migrate up|down")
		}
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return errors.New("DATABASE_URL is required")
		}
		migrationCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		switch os.Args[2] {
		case "up":
			return migrations.Up(migrationCtx, dsn)
		case "down":
			return migrations.Down(migrationCtx, dsn)
		default:
			return errors.New("migration direction must be up or down")
		}
	}
	if command != "serve" && command != "user" {
		return errors.New("unknown command, run logbook help")
	}
	// Account maintenance only needs PostgreSQL, even when the API uses S3.
	if command == "user" {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return errors.New("DATABASE_URL is required")
		}
		pool, err := openDatabase(ctx, dsn, 2)
		if err != nil {
			return err
		}
		defer pool.Close()
		return manageUser(ctx, service.NewAuth(repository.New(pool), 12*time.Hour), os.Args[2:])
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := openDatabase(ctx, cfg.DatabaseURL, cfg.DatabaseMaxConns)
	if err != nil {
		return err
	}
	defer pool.Close()
	var store storage.Store
	if cfg.StorageDriver == "local" {
		local, err := storage.NewLocal(cfg.LocalStoragePath)
		if err != nil {
			return err
		}
		defer local.Close()
		store = local
	} else {
		startupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		store, err = storage.NewS3(startupCtx, storage.S3Config{Bucket: cfg.S3Bucket, Region: cfg.S3Region, Endpoint: cfg.S3Endpoint, PathStyle: cfg.S3PathStyle})
		cancel()
		if err != nil {
			return err
		}
	}
	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err = store.Check(startupCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("storage unavailable: %w", err)
	}
	// Failing before listening prevents serving against a missing schema.
	var schemaReady bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('logbooks') IS NOT NULL AND to_regclass('sessions') IS NOT NULL").Scan(&schemaReady); err != nil {
		return err
	}
	if !schemaReady {
		return errors.New("database schema is missing, run logbook migrate up")
	}
	repo := repository.New(pool)
	books := service.NewLogbooks(repo, store, service.Limits{MaxPhotosPerUpload: cfg.MaxPhotosPerUpload, MaxPhotosPerLogbook: cfg.MaxPhotosPerLogbook, MaxPhotoBytes: cfg.MaxPhotoBytes, MaxImagePixels: cfg.MaxImagePixels}, logger)
	auth := service.NewAuth(repo, cfg.SessionTTL)
	handler := httpapi.New(books, auth, httpapi.Options{Logger: logger, AllowedOrigins: cfg.AllowedOrigins, MaxRequestBytes: cfg.MaxRequestBytes, MaxConcurrentUploads: cfg.MaxConcurrentUploads, RequestTimeout: cfg.RequestTimeout, Ready: func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		return store.Check(ctx)
	}})
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.RequestTimeout, WriteTimeout: cfg.RequestTimeout + 10*time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); cleanupLoop(workerCtx, books, auth, logger) }()
	defer func() { stopWorker(); <-workerDone }()
	errorsCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "address", cfg.HTTPAddr, "environment", cfg.Environment, "storage", cfg.StorageDriver)
		errorsCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}

func openDatabase(ctx context.Context, dsn string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	cfg.MaxConns = maxConns
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database unavailable: %w", err)
	}
	return pool, nil
}

func cleanupLoop(ctx context.Context, books *service.Logbooks, auth *service.Auth, logger *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		workCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		if err := books.Cleanup(workCtx); err != nil && ctx.Err() == nil {
			logger.Error("object cleanup failed", "error", err)
		}
		if err := auth.Cleanup(workCtx); err != nil && ctx.Err() == nil {
			logger.Error("session cleanup failed", "error", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func manageUser(ctx context.Context, auth *service.Auth, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: logbook user create|reset-password|disable --username NAME")
	}
	command := args[0]
	flags := flag.NewFlagSet("user "+command, flag.ContinueOnError)
	username := flags.String("username", "", "unique login name")
	name := flags.String("name", "", "operator display name (create only)")
	role := flags.String("role", "operator", "operator or admin (create only)")
	stdin := flags.Bool("password-stdin", false, "read password from stdin for automation")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if command == "disable" {
		return auth.Disable(ctx, *username)
	}
	if command != "create" && command != "reset-password" {
		return errors.New("unknown user command")
	}
	var password []byte
	var err error
	if *stdin {
		password, err = io.ReadAll(io.LimitReader(os.Stdin, 74))
		password = []byte(strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r"))
	} else if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password (at least 12 characters): ")
		password, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		return errors.New("use an interactive terminal or --password-stdin")
	}
	if err != nil {
		return err
	}
	defer clear(password)
	if command == "create" {
		err = auth.Provision(ctx, *username, *name, string(password), *role)
	} else {
		err = auth.ResetPassword(ctx, *username, string(password))
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Account updated.")
	return nil
}
