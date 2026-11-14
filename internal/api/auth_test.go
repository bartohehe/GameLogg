package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bartohehe/GameLogg/internal/auth"
	"github.com/bartohehe/GameLogg/internal/store"
)

// fakeUsers to magazyn kont w pamięci, zachowujący się jak store.Store.
type fakeUsers struct {
	mu    sync.Mutex
	next  int64
	users map[int64]store.User
}

func newFakeUsers() *fakeUsers {
	return &fakeUsers{next: 1, users: map[int64]store.User{}}
}

func (f *fakeUsers) CreateUser(_ context.Context, username, email, hash string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return store.User{}, store.ErrEmailTaken
		}
		if strings.EqualFold(u.Username, username) {
			return store.User{}, store.ErrUsernameTaken
		}
	}
	u := store.User{ID: f.next, Username: username, Email: email, PasswordHash: hash, CreatedAt: time.Now()}
	f.users[u.ID] = u
	f.next++
	return u, nil
}

func (f *fakeUsers) UserByEmail(_ context.Context, email string) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return store.User{}, store.ErrNotFound
}

func (f *fakeUsers) UserByID(_ context.Context, id int64) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

const testSecret = "0123456789abcdef0123456789abcdef"

func authRouter(t *testing.T) (http.Handler, *fakeUsers) {
	t.Helper()
	users := newFakeUsers()
	h := NewRouter(Deps{
		Log:      testLogger(),
		DB:       fakeDB{},
		Users:    users,
		Auth:     AuthDeps{Enabled: true, Tokens: auth.NewTokens(testSecret, time.Hour)},
		Features: Features{AuthEnabled: true, MediaEnabled: true},
		Version:  "test",
	})
	return h, users
}

func send(t *testing.T, h http.Handler, method, target string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func sessionFrom(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatal("brak ciasteczka sesji")
	return nil
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) apiError {
	t.Helper()
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("odpowiedź nie jest błędem JSON: %v", err)
	}
	return body.Error
}

var validReg = registerRequest{Username: "bartek", Email: "bartek@example.com", Password: "dobre-haslo"}

func TestRegisterLoginMeLogout(t *testing.T) {
	h, _ := authRouter(t)

	rec := send(t, h, http.MethodPost, "/api/auth/register", validReg)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d: %s", rec.Code, rec.Body)
	}
	c := sessionFrom(t, rec)
	if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Secure {
		t.Errorf("ciasteczko: HttpOnly=%v SameSite=%v Secure=%v", c.HttpOnly, c.SameSite, c.Secure)
	}

	// Rejestracja loguje od razu.
	rec = send(t, h, http.MethodGet, "/api/me", nil, c)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	var me userDTO
	if err := json.NewDecoder(rec.Body).Decode(&me); err != nil || me.Username != "bartek" {
		t.Errorf("me = %+v, %v", me, err)
	}

	// Logowanie z innym zapisem emaila.
	rec = send(t, h, http.MethodPost, "/api/auth/login", loginRequest{Email: " Bartek@Example.com ", Password: "dobre-haslo"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", rec.Code, rec.Body)
	}
	c = sessionFrom(t, rec)

	rec = send(t, h, http.MethodPost, "/api/auth/logout", nil, c)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", rec.Code)
	}
	if out := sessionFrom(t, rec); out.MaxAge >= 0 {
		t.Errorf("logout nie kasuje ciasteczka: MaxAge = %d", out.MaxAge)
	}
}

func TestSecureCookieBehindHTTPSProxy(t *testing.T) {
	h, _ := authRouter(t)
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(validReg)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if c := sessionFrom(t, rec); !c.Secure {
		t.Error("za proxy HTTPS ciasteczko musi mieć Secure")
	}
}

func TestLoginErrors(t *testing.T) {
	h, _ := authRouter(t)
	if rec := send(t, h, http.MethodPost, "/api/auth/register", validReg); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	tests := []struct {
		name       string
		req        loginRequest
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"złe hasło", loginRequest{"bartek@example.com", "zle-haslo"}, 401, "invalid_credentials", ""},
		{"nieznany email", loginRequest{"nikt@example.com", "dobre-haslo"}, 401, "invalid_credentials", ""},
		{"pusty email", loginRequest{"", "x"}, 400, "missing_email", "email"},
		{"puste hasło", loginRequest{"bartek@example.com", ""}, 400, "missing_password", "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/auth/login", tt.req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d", rec.Code)
			}
			e := decodeErr(t, rec)
			if e.Code != tt.wantCode || e.Field != tt.wantField {
				t.Errorf("błąd = %+v", e)
			}
			if tt.wantCode == "invalid_credentials" && e.Message != "Email albo hasło się nie zgadzają. Sprawdź oba pola." {
				t.Errorf("komunikat = %q", e.Message)
			}
		})
	}
}

func TestRegisterValidation(t *testing.T) {
	h, _ := authRouter(t)
	if rec := send(t, h, http.MethodPost, "/api/auth/register", validReg); rec.Code != http.StatusCreated {
		t.Fatal(rec.Body)
	}
	tests := []struct {
		name       string
		req        registerRequest
		wantStatus int
		wantCode   string
		wantField  string
	}{
		{"krótka nazwa", registerRequest{"ab", "a@example.com", "dobre-haslo"}, 400, "invalid_username", "username"},
		{"spacja w nazwie", registerRequest{"jan kowalski", "a@example.com", "dobre-haslo"}, 400, "invalid_username", "username"},
		{"zarezerwowana nazwa", registerRequest{"Owner", "a@example.com", "dobre-haslo"}, 409, "username_taken", "username"},
		{"zły email", registerRequest{"jan", "jan@", "dobre-haslo"}, 400, "invalid_email", "email"},
		{"email bez domeny", registerRequest{"jan", "jan@localhost", "dobre-haslo"}, 400, "invalid_email", "email"},
		{"krótkie hasło", registerRequest{"jan", "jan@example.com", "krotkie"}, 400, "invalid_password", "password"},
		{"zajęty email", registerRequest{"jan", "BARTEK@example.com", "dobre-haslo"}, 409, "email_taken", "email"},
		{"zajęta nazwa", registerRequest{"Bartek", "jan@example.com", "dobre-haslo"}, 409, "username_taken", "username"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, h, http.MethodPost, "/api/auth/register", tt.req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body)
			}
			if e := decodeErr(t, rec); e.Code != tt.wantCode || e.Field != tt.wantField {
				t.Errorf("błąd = %+v", e)
			}
		})
	}
}

func TestMeRequiresSession(t *testing.T) {
	h, users := authRouter(t)

	rec := send(t, h, http.MethodGet, "/api/me", nil)
	if rec.Code != http.StatusUnauthorized || decodeErr(t, rec).Code != "unauthorized" {
		t.Errorf("bez ciasteczka: %d", rec.Code)
	}

	rec = send(t, h, http.MethodGet, "/api/me", nil, &http.Cookie{Name: sessionCookie, Value: "smieci"})
	if rec.Code != http.StatusUnauthorized || decodeErr(t, rec).Code != "session_expired" {
		t.Errorf("zły token: %d", rec.Code)
	}

	// Ważny token, ale konto zniknęło.
	tok, _, _ := auth.NewTokens(testSecret, time.Hour).Issue(999)
	rec = send(t, h, http.MethodGet, "/api/me", nil, &http.Cookie{Name: sessionCookie, Value: tok})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("usunięte konto: %d", rec.Code)
	}
	_ = users
}

func TestSingleUserMode(t *testing.T) {
	users := newFakeUsers()
	owner, _ := users.CreateUser(context.Background(), store.OwnerUsername, "owner@localhost", "")
	h := NewRouter(Deps{
		Log: testLogger(), DB: fakeDB{}, Users: users,
		Auth:     AuthDeps{Enabled: false, OwnerID: owner.ID},
		Features: Features{AuthEnabled: false},
	})

	rec := send(t, h, http.MethodGet, "/api/me", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d", rec.Code)
	}
	var me userDTO
	_ = json.NewDecoder(rec.Body).Decode(&me)
	if me.ID != owner.ID || me.Username != store.OwnerUsername {
		t.Errorf("me = %+v", me)
	}

	for _, path := range []string{"/api/auth/login", "/api/auth/register", "/api/auth/logout"} {
		rec := send(t, h, http.MethodPost, path, validReg)
		if rec.Code != http.StatusNotFound || decodeErr(t, rec).Code != "auth_disabled" {
			t.Errorf("%s: status = %d", path, rec.Code)
		}
	}
}

func TestConfigIsPublic(t *testing.T) {
	h := NewRouter(Deps{
		Log: testLogger(), DB: fakeDB{}, Users: newFakeUsers(),
		Auth:     AuthDeps{Enabled: true, Tokens: auth.NewTokens(testSecret, time.Hour)},
		Features: Features{AuthEnabled: true, MediaEnabled: false, TranslationEnabled: true, IGDBConfigured: true},
		Version:  "1.0.0",
	})
	rec := send(t, h, http.MethodGet, "/api/config", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&got)
	want := map[string]any{
		"authEnabled": true, "mediaEnabled": false, "translationEnabled": true,
		"igdbConfigured": true, "tmdbConfigured": false, "version": "1.0.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, oczekiwano %v", k, got[k], v)
		}
	}
}

func TestMutationsRequireJSONContentType(t *testing.T) {
	h, _ := authRouter(t)
	for _, ct := range []string{"", "application/x-www-form-urlencoded", "text/plain"} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", strings.NewReader("a=b"))
		if ct != "" {
			req.Header.Set("Content-Type", ct)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("Content-Type %q: status = %d", ct, rec.Code)
		}
	}
}

func TestInvalidJSON(t *testing.T) {
	h, _ := authRouter(t)
	for _, body := range []string{"{", `{"email":"a"} {"x":1}`, "[]"} {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || decodeErr(t, rec).Code != "invalid_json" {
			t.Errorf("body %q: status = %d", body, rec.Code)
		}
	}
}

func (f *fakeUsers) UpdatePassword(_ context.Context, id int64, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.ErrNotFound
	}
	now := time.Now()
	u.PasswordHash, u.PasswordChangedAt = hash, &now
	f.users[id] = u
	return nil
}

func TestChangePassword(t *testing.T) {
	h, users := authRouter(t)
	c := sessionFrom(t, send(t, h, http.MethodPost, "/api/auth/register", validReg))

	tests := []struct {
		name  string
		body  changePasswordRequest
		code  string
		field string
	}{
		{"brak obecnego", changePasswordRequest{"", "nowe-haslo-123"}, "missing_password", "currentPassword"},
		{"złe obecne", changePasswordRequest{"zle-haslo", "nowe-haslo-123"}, "wrong_password", "currentPassword"},
		{"za krótkie nowe", changePasswordRequest{"dobre-haslo", "krotkie"}, "invalid_password", "newPassword"},
		{"takie samo", changePasswordRequest{"dobre-haslo", "dobre-haslo"}, "same_password", "newPassword"},
	}
	for _, tt := range tests {
		rec := send(t, h, http.MethodPut, "/api/me/password", tt.body, c)
		if e := decodeErr(t, rec); rec.Code != http.StatusBadRequest || e.Code != tt.code || e.Field != tt.field {
			t.Errorf("%s: %d %+v", tt.name, rec.Code, e)
		}
	}

	rec := send(t, h, http.MethodPut, "/api/me/password", changePasswordRequest{"dobre-haslo", "nowe-haslo-123"}, c)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("zmiana: %d %s", rec.Code, rec.Body)
	}
	fresh := sessionFrom(t, rec)

	// Bieżąca sesja dostaje nowe ciasteczko i działa dalej.
	if rec := send(t, h, http.MethodGet, "/api/me", nil, fresh); rec.Code != http.StatusOK {
		t.Errorf("nowa sesja: %d", rec.Code)
	}
	// Logowanie: stare hasło już nie działa, nowe tak.
	if rec := send(t, h, http.MethodPost, "/api/auth/login", loginRequest{validReg.Email, "dobre-haslo"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("stare hasło: %d", rec.Code)
	}
	if rec := send(t, h, http.MethodPost, "/api/auth/login", loginRequest{validReg.Email, "nowe-haslo-123"}); rec.Code != http.StatusOK {
		t.Errorf("nowe hasło: %d", rec.Code)
	}

	// Sesja z tokenem wystawionym przed zmianą hasła wygasa (inne urządzenie).
	users.mu.Lock()
	u := users.users[1]
	later := time.Now().Add(2 * time.Second)
	u.PasswordChangedAt = &later
	users.users[1] = u
	users.mu.Unlock()
	if rec := send(t, h, http.MethodGet, "/api/me", nil, fresh); rec.Code != http.StatusUnauthorized || decodeErr(t, rec).Code != "session_expired" {
		t.Errorf("sesja sprzed zmiany hasła: %d", rec.Code)
	}
}

func TestChangePasswordWithoutAuth(t *testing.T) {
	users := newFakeUsers()
	owner, _ := users.CreateUser(context.Background(), store.OwnerUsername, "owner@localhost", "")
	h := NewRouter(Deps{Log: testLogger(), DB: fakeDB{}, Users: users, Auth: AuthDeps{OwnerID: owner.ID}})
	rec := send(t, h, http.MethodPut, "/api/me/password", changePasswordRequest{"a", "nowe-haslo-123"})
	if rec.Code != http.StatusNotFound || decodeErr(t, rec).Code != "auth_disabled" {
		t.Errorf("bez logowania: %d", rec.Code)
	}
}
