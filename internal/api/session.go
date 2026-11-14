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
		expired := func() {
			clearSessionCookie(w, r)
			writeError(w, http.StatusUnauthorized, "session_expired", "Sesja wygasła. Zaloguj się ponownie.")
		}
		sess, err := s.auth.Tokens.ParseSession(c.Value)
		if err != nil {
			expired()
			return
		}
		// Konto mogło zostać usunięte po wystawieniu tokenu.
		u, err := s.users.UserByID(r.Context(), sess.UserID)
		if errors.Is(err, store.ErrNotFound) {
			expired()
			return
		}
		if err != nil {
			writeInternal(w, r, s.log, err)
			return
		}
		// Zmiana hasła unieważnia sesje wystawione przed nią. Token ma iat
		// z dokładnością do sekundy, więc porównujemy pełne sekundy.
		if u.PasswordChangedAt != nil && sess.IssuedAt.Before(u.PasswordChangedAt.Truncate(time.Second)) {
			expired()
			return
		}
		next.ServeHTTP(w, r.WithContext(withUserID(r.Context(), sess.UserID)))
	})
}
