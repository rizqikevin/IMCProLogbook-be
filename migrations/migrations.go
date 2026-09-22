package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed *.sql
var files embed.FS

func Up(ctx context.Context, databaseURL string) error {
	return run(ctx, databaseURL, false)
}

func Down(ctx context.Context, databaseURL string) error {
	return run(ctx, databaseURL, true)
}

func run(ctx context.Context, databaseURL string, down bool) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("initialize migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("initialize migrations: %w", err)
	}
	if down {
		_, err = provider.Down(ctx)
	} else {
		_, err = provider.Up(ctx)
	}
	if err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}
