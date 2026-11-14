package api

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/bartohehe/gamelogg/internal/auth"
	"github.com/bartohehe/gamelogg/internal/store"
)

type userDTO struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

func toUserDTO(u store.User) userDTO {
	return userDTO{ID: u.ID, Username: u.Username, Email: u.Email, CreatedAt: u.CreatedAt}
}

// authEnabledOnly ukrywa endpointy logowania w trybie jednego użytkownika.
func (s *server) authEnabledOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Enabled {
			writeError(w, http.StatusNotFound, "auth_disabled",
				"Logowanie jest wyłączone na tym serwerze (AUTH_ENABLED=false).")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func validUsername(u string) bool {
	n := len([]rune(u))
	if n < 3 || n > 32 {
		return false
	}
	for _, c := range u {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '-' && c != '.' {
			return false
		}
	}
	return true
}

func validEmail(e string) bool {
	if len(e) > 254 {
		return false
	}
	addr, err := mail.ParseAddress(e)
	return err == nil && addr.Address == e && strings.Contains(e[strings.LastIndex(e, "@"):], ".")
}

func passwordMessage(err error) string {
	if errors.Is(err, auth.ErrPasswordTooLong) {
		return "Hasło jest za długie: najwyżej 72 bajty."
	}
	return "Hasło musi mieć co najmniej 8 znaków."
}

func (s *server) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)

	switch {
	case !validUsername(req.Username):
		writeFieldError(w, http.StatusBadRequest, "invalid_username", "username",
			"Nazwa użytkownika musi mieć od 3 do 32 znaków: litery, cyfry, kropka, myślnik albo podkreślnik.")
		return
	case strings.EqualFold(req.Username, store.OwnerUsername):
		writeFieldError(w, http.StatusConflict, "username_taken", "username",
			"Ta nazwa jest zarezerwowana. Wybierz inną.")
		return
	case !validEmail(req.Email):
		writeFieldError(w, http.StatusBadRequest, "invalid_email", "email",
			"Wpisz poprawny adres email, np. jan@example.com.")
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeFieldError(w, http.StatusBadRequest, "invalid_password", "password", passwordMessage(err))
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	u, err := s.users.CreateUser(r.Context(), req.Username, req.Email, hash)
	switch {
	case errors.Is(err, store.ErrEmailTaken):
		writeFieldError(w, http.StatusConflict, "email_taken", "email",
			"Ten email ma już konto. Zaloguj się albo użyj innego adresu.")
		return
	case errors.Is(err, store.ErrUsernameTaken):
		writeFieldError(w, http.StatusConflict, "username_taken", "username",
			"Ta nazwa jest zajęta. Wybierz inną.")
		return
	case err != nil:
		writeInternal(w, r, s.log, err)
		return
	}

	if !s.startSession(w, r, u.ID) {
		return
	}
	s.log.InfoContext(r.Context(), "user registered", "user_id", u.ID)
	writeJSON(w, http.StatusCreated, toUserDTO(u))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeFieldError(w, http.StatusBadRequest, "missing_email", "email", "Wpisz email.")
		return
	}
	if req.Password == "" {
		writeFieldError(w, http.StatusBadRequest, "missing_password", "password", "Wpisz hasło.")
		return
	}

	u, err := s.users.UserByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeInternal(w, r, s.log, err)
		return
	}
	if err != nil {
		auth.SpendCheckTime(req.Password)
	}
	if err != nil || !auth.CheckPassword(u.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials",
			"Email albo hasło się nie zgadzają. Sprawdź oba pola.")
		return
	}

	if !s.startSession(w, r, u.ID) {
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}

func (s *server) startSession(w http.ResponseWriter, r *http.Request, userID int64) bool {
	token, exp, err := s.auth.Tokens.Issue(userID)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return false
	}
	setSessionCookie(w, r, token, exp)
	return true
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.users.UserByID(r.Context(), userIDFrom(r.Context()))
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(u))
}
