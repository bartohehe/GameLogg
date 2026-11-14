package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bartohehe/GameLogg/internal/store"
)

const sessionCookie = "gamelogg_session"

type ctxKey int

const userIDKey ctxKey = iota

// userIDFrom zwraca identyfikator zalogowanego użytkownika. Wołane tylko
// w handlerach za requireUser, więc identyfikator zawsze istnieje.
func userIDFrom(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey).(int64)
	return id
}

func withUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

// isHTTPS mówi, czy klient połączył się przez HTTPS: bezpośrednio albo przez
// reverse proxy (Caddy, Traefik), które ustawia X-Forwarded-Proto.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// requireUser ustala użytkownika żądania. Przy AUTH_ENABLED=false każde
// żądanie działa w kontekście konta owner. W przeciwnym razie wymagane jest
// ważne ciasteczko sesji wskazujące istniejące konto.
func (s *server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Enabled {
			next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), s.auth.OwnerID)))
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Zaloguj się, żeby zobaczyć tę stronę.")
			return
		}
		id, err := s.auth.Tokens.Parse(c.Value)
		if err != nil {
			clearSessionCookie(w, r)
			writeError(w, http.StatusUnauthorized, "session_expired", "Sesja wygasła. Zaloguj się ponownie.")
			return
		}
		// Konto mogło zostać usunięte po wystawieniu tokenu.
		if _, err := s.users.UserByID(r.Context(), id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				clearSessionCookie(w, r)
				writeError(w, http.StatusUnauthorized, "session_expired", "Sesja wygasła. Zaloguj się ponownie.")
				return
			}
			writeInternal(w, r, s.log, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), id)))
	})
}
