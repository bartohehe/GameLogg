package store

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/bartohehe/gamelogg/internal/migrations"
)

// openTestStore łączy się z bazą testową wskazaną przez TEST_DATABASE_URL
// (np. serwis postgres z docker-compose.dev.yml). Testy integracyjne są
// pomijane flagą -short albo gdy zmienna nie jest ustawiona.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	if testing.Short() {
		t.Skip("test integracyjny pominięty przez -short")
	}
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL nie jest ustawione (make test-db uruchamia bazę testową)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := Open(ctx, url, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestMigrationsUpDownUp(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	db := s.SQLDB()
	defer db.Close()

	// Czysty start: cofnij wszystko, co mogło zostać po poprzednim przebiegu.
	if err := migrations.Reset(ctx, db); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if err := s.Migrate(ctx, log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := migrations.Reset(ctx, db); err != nil {
		t.Fatalf("Reset po Up: %v", err)
	}
	if err := s.Migrate(ctx, log); err != nil {
		t.Fatalf("ponowne Migrate: %v", err)
	}

	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public'
		   AND table_name IN ('users','games','user_games','category_rankings','user_media','translation_cache')`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 {
		t.Errorf("tabel = %d, oczekiwano 6", n)
	}
}

func TestPing(t *testing.T) {
	s := openTestStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}
