// Package store zawiera całe SQL aplikacji. Każde zapytanie o dane użytkownika
// filtruje po user_id przekazanym przez wywołującego.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/bartohehe/gamelogg/internal/migrations"
)

// Store opakowuje pulę połączeń z Postgresem.
type Store struct {
	pool *pgxpool.Pool
}

// Open łączy się z bazą i czeka, aż odpowie. Przy starcie kontenerów Postgres
// bywa gotowy chwilę po aplikacji, więc próbujemy ponownie do upływu ctx.
func Open(ctx context.Context, databaseURL string, log *slog.Logger) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pula połączeń: %w", err)
	}

	backoff := 500 * time.Millisecond
	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			break
		}
		log.Warn("database not ready", "attempt", attempt, "err", err)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("baza danych nie odpowiada: %w", errors.Join(err, ctx.Err()))
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 5*time.Second)
	}
	return &Store{pool: pool}, nil
}

// Migrate stosuje wbudowane migracje goose.
func (s *Store) Migrate(ctx context.Context, log *slog.Logger) error {
	db := stdlib.OpenDBFromPool(s.pool)
	defer db.Close()
	return migrations.Up(ctx, db, log)
}

// SQLDB zwraca *sql.DB na tej samej puli (dla narzędzi wymagających database/sql).
// Wywołujący zamyka zwrócony obiekt.
func (s *Store) SQLDB() *sql.DB {
	return stdlib.OpenDBFromPool(s.pool)
}

// Ping sprawdza, czy baza odpowiada.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close zamyka pulę połączeń.
func (s *Store) Close() {
	s.pool.Close()
}
