// Command gamelogg uruchamia serwer GameLogg: API pod /api i wbudowany frontend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bartohehe/gamelogg/internal/api"
	"github.com/bartohehe/gamelogg/internal/auth"
	"github.com/bartohehe/gamelogg/internal/config"
	"github.com/bartohehe/gamelogg/internal/igdb"
	"github.com/bartohehe/gamelogg/internal/spine"
	"github.com/bartohehe/gamelogg/internal/store"
	"github.com/bartohehe/gamelogg/web"
)

// version jest wstrzykiwana przy buildzie: -ldflags "-X main.version=1.0.0".
var version = "dev"

// sessionTTL to czas życia sesji po zalogowaniu.
const sessionTTL = 30 * 24 * time.Hour

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			os.Exit(healthcheck())
		case "version", "--version":
			fmt.Println(version)
			return
		}
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	log := newLogger(cfg.Dev)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("starting gamelogg", "version", version, "port", cfg.Port,
		"auth", cfg.AuthEnabled, "media", cfg.MediaEnabled, "translation", cfg.TranslationEnabled,
		"igdb", cfg.IGDBConfigured(), "tmdb", cfg.TMDBConfigured())

	openCtx, cancelOpen := context.WithTimeout(ctx, 60*time.Second)
	db, err := store.Open(openCtx, cfg.DatabaseURL, log)
	cancelOpen()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx, log); err != nil {
		return err
	}

	authDeps := api.AuthDeps{Enabled: cfg.AuthEnabled}
	if cfg.AuthEnabled {
		authDeps.Tokens = auth.NewTokens(cfg.JWTSecret, sessionTTL)
	} else {
		ownerID, err := db.EnsureOwner(ctx)
		if err != nil {
			return err
		}
		authDeps.OwnerID = ownerID
		log.Info("single-user mode: every request acts as owner", "user_id", ownerID)
	}

	// Interfejs musi zostać nil (a nie typowany nil), gdy IGDB nie jest skonfigurowane.
	var games api.GameSource
	if cfg.IGDBConfigured() {
		games = igdb.New(cfg.IGDBClientID, cfg.IGDBClientSecret)
	}

	srv := &http.Server{
		Addr: cfg.Addr(),
		Handler: api.NewRouter(api.Deps{
			Log:     log,
			DB:      db,
			Users:   db,
			Library: db,
			Games:   games,
			Spines:  spine.NewExtractor(),
			Auth:    authDeps,
			Features: api.Features{
				AuthEnabled:        cfg.AuthEnabled,
				MediaEnabled:       cfg.MediaEnabled,
				TranslationEnabled: cfg.TranslationEnabled,
				IGDBConfigured:     cfg.IGDBConfigured(),
				TMDBConfigured:     cfg.TMDBConfigured(),
			},
			Version: version,
			Web:     web.Dist(),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serwer HTTP: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("zamykanie serwera: %w", err)
	}
	return nil
}

func newLogger(dev bool) *slog.Logger {
	if dev {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

// healthcheck odpytuje lokalny /api/health. Obraz distroless nie ma curla,
// więc healthcheck kontenera woła samą binarkę: `gamelogg healthcheck`.
func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "status", resp.StatusCode)
		return 1
	}
	return 0
}
