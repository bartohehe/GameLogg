// Package migrations trzyma schemat bazy jako pliki SQL goose wbudowane w binarkę.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

func provider(db *sql.DB) (*goose.Provider, error) {
	p, err := goose.NewProvider(goose.DialectPostgres, db, FS)
	if err != nil {
		return nil, fmt.Errorf("goose: %w", err)
	}
	return p, nil
}

// Up stosuje wszystkie brakujące migracje.
func Up(ctx context.Context, db *sql.DB, log *slog.Logger) error {
	p, err := provider(db)
	if err != nil {
		return err
	}
	results, err := p.Up(ctx)
	if err != nil {
		return fmt.Errorf("migracje: %w", err)
	}
	for _, r := range results {
		log.Info("migration applied", "version", r.Source.Version, "file", r.Source.Path, "duration", r.Duration)
	}
	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("wersja schematu: %w", err)
	}
	log.Info("database schema ready", "version", v)
	return nil
}

// Reset cofa wszystkie migracje. Używane tylko w testach.
func Reset(ctx context.Context, db *sql.DB) error {
	p, err := provider(db)
	if err != nil {
		return err
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		return fmt.Errorf("cofanie migracji: %w", err)
	}
	return nil
}
