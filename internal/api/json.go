package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
)

// errorBody to jedyny format błędów API: {"error": {"code": "...", "message": "..."}}.
// Pole field (opcjonalne) wskazuje pole formularza, którego dotyczy błąd.
type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

const maxBodyBytes = 64 << 10

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Default().Error("encode response", "err", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message}})
}

func writeFieldError(w http.ResponseWriter, status int, code, field, message string) {
	writeJSON(w, status, errorBody{Error: apiError{Code: code, Message: message, Field: field}})
}

func writeInternal(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.ErrorContext(r.Context(), "internal error", "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal",
		"Coś poszło nie tak po stronie serwera. Szczegóły są w logach kontenera app.")
}

// decodeJSON czyta ciało żądania do v. Przy błędzie sam wysyła odpowiedź
// i zwraca false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Za duże żądanie.")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "Treść żądania nie jest poprawnym JSON-em.")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "Treść żądania nie jest poprawnym JSON-em.")
		return false
	}
	return true
}

// requireJSON odrzuca żądania zmieniające stan, które nie mają Content-Type
// application/json (także te bez ciała). Formularz HTML ani "prosty" fetch
// z obcej strony nie ustawią takiego nagłówka bez zapytania CORS, więc to
// ochrona przed CSRF niezależna od ciasteczka SameSite=Lax.
func requireJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
					"Żądanie musi mieć Content-Type: application/json.")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
