// Package api zawiera handlery HTTP, middleware i DTO.
package api

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/bartohehe/gamelogg/internal/auth"
	"github.com/bartohehe/gamelogg/internal/store"
)

// Pinger sprawdza dostępność bazy danych.
type Pinger interface {
	Ping(ctx context.Context) error
}

// UserStore to operacje na kontach używane przez API.
type UserStore interface {
	CreateUser(ctx context.Context, username, email, passwordHash string) (store.User, error)
	UserByEmail(ctx context.Context, email string) (store.User, error)
	UserByID(ctx context.Context, id int64) (store.User, error)
}

// Features to flagi serwera widoczne publicznie przez /api/config.
type Features struct {
	AuthEnabled        bool
	MediaEnabled       bool
	TranslationEnabled bool
	IGDBConfigured     bool
	TMDBConfigured     bool
}

// AuthDeps opisuje tryb logowania.
type AuthDeps struct {
	Enabled bool
	// Tokens jest wymagane, gdy Enabled.
	Tokens *auth.Tokens
	// OwnerID to konto używane przy Enabled == false.
	OwnerID int64
}

// Deps to zależności routera. Pola są interfejsami, żeby testy mogły
// podstawić atrapy bez sieci i bazy.
type Deps struct {
	Log      *slog.Logger
	DB       Pinger
	Users    UserStore
	Auth     AuthDeps
	Features Features
	Version  string
	// Web to zbudowany frontend (zawartość web/dist). Może nie zawierać
	// index.html, jeśli binarkę zbudowano bez frontendu.
	Web fs.FS
}

type server struct {
	log      *slog.Logger
	db       Pinger
	users    UserStore
	auth     AuthDeps
	features Features
	version  string
}

// NewRouter składa całe API pod /api oraz frontend z fallbackiem SPA.
func NewRouter(d Deps) http.Handler {
	s := &server{
		log:      d.Log,
		db:       d.DB,
		users:    d.Users,
		auth:     d.Auth,
		features: d.Features,
		version:  d.Version,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(d.Log))
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Use(requireJSON)

		r.Get("/health", s.health)
		r.Get("/config", s.config)

		r.Route("/auth", func(r chi.Router) {
			r.Use(s.authEnabledOnly)
			r.Post("/register", s.register)
			r.Post("/login", s.login)
			r.Post("/logout", s.logout)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)
			r.Get("/me", s.me)
		})

		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "not_found", "Nie ma takiego endpointu.")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Ten endpoint nie obsługuje tej metody.")
		})
	})

	r.Handle("/*", newSPAHandler(d.Web))
	return r
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		s.log.Warn("health: database ping failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "database_unavailable",
			"Baza danych nie odpowiada. Sprawdź, czy kontener postgres działa.")
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.version})
}

type configResponse struct {
	AuthEnabled        bool   `json:"authEnabled"`
	MediaEnabled       bool   `json:"mediaEnabled"`
	TranslationEnabled bool   `json:"translationEnabled"`
	IGDBConfigured     bool   `json:"igdbConfigured"`
	TMDBConfigured     bool   `json:"tmdbConfigured"`
	Version            string `json:"version"`
}

func (s *server) config(w http.ResponseWriter, _ *http.Request) {
	f := s.features
	writeJSON(w, http.StatusOK, configResponse{
		AuthEnabled:        f.AuthEnabled,
		MediaEnabled:       f.MediaEnabled,
		TranslationEnabled: f.TranslationEnabled,
		IGDBConfigured:     f.IGDBConfigured,
		TMDBConfigured:     f.TMDBConfigured,
		Version:            s.version,
	})
}
