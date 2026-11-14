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
)

// Pinger sprawdza dostępność bazy danych.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps to zależności routera. Pola są interfejsami, żeby testy mogły
// podstawić atrapy bez sieci i bazy.
type Deps struct {
	Log     *slog.Logger
	DB      Pinger
	Version string
	// Web to zbudowany frontend (zawartość web/dist). Może nie zawierać
	// index.html, jeśli binarkę zbudowano bez frontendu.
	Web fs.FS
}

// NewRouter składa całe API pod /api oraz frontend z fallbackiem SPA.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(d.Log))
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler(d))
		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusNotFound, "not_found", "Nie ma takiego endpointu.")
		})
		r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Ten endpoint nie obsługuje tej metody.")
		})
	})

	spa := newSPAHandler(d.Web)
	r.Handle("/*", spa)
	return r
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

func healthHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			d.Log.Warn("health: database ping failed", "err", err)
			writeError(w, http.StatusServiceUnavailable, "database_unavailable",
				"Baza danych nie odpowiada. Sprawdź, czy kontener postgres działa.")
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: d.Version})
	}
}
