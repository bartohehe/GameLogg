package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

var webFS = fstest.MapFS{
	"index.html":        {Data: []byte("<!doctype html><title>GameLogg</title>")},
	"assets/app-abc.js": {Data: []byte("console.log(1)")},
	"favicon.svg":       {Data: []byte("<svg/>")},
}

func newTestRouter(db Pinger, web fstest.MapFS) http.Handler {
	d := Deps{Log: testLogger(), DB: db, Users: newFakeUsers(), Auth: AuthDeps{Enabled: false, OwnerID: 1}, Version: "1.2.3"}
	if web != nil {
		d.Web = web
	}
	return NewRouter(d)
}

func do(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestHealthOK(t *testing.T) {
	rec := do(t, newTestRouter(fakeDB{}, webFS), http.MethodGet, "/api/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Version != "1.2.3" {
		t.Errorf("body = %+v", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q", ct)
	}
}

func TestHealthDatabaseDown(t *testing.T) {
	rec := do(t, newTestRouter(fakeDB{err: errors.New("down")}, webFS), http.MethodGet, "/api/health")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "database_unavailable" || body.Error.Message == "" {
		t.Errorf("body = %+v", body)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	rec := do(t, newTestRouter(fakeDB{}, webFS), http.MethodGet, "/api/nope")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	var body errorBody
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("odpowiedź nie jest JSON-em: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("code = %q", body.Error.Code)
	}
}

func TestSPA(t *testing.T) {
	h := newTestRouter(fakeDB{}, webFS)
	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{"root", "/", 200, "<title>GameLogg</title>", "no-cache"},
		{"trasa klienta", "/library", 200, "<title>GameLogg</title>", "no-cache"},
		{"zagnieżdżona trasa klienta", "/game/1942", 200, "<title>GameLogg</title>", "no-cache"},
		{"plik statyczny", "/favicon.svg", 200, "<svg/>", ""},
		{"asset z hashem", "/assets/app-abc.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"brakujący asset", "/assets/missing.js", 404, "", ""},
		// Ścieżka jest czyszczona, więc nie wychodzi poza dist: dostajemy zwykły fallback SPA.
		{"próba wyjścia poza katalog", "/../../etc/passwd", 200, "<title>GameLogg</title>", "no-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h, http.MethodGet, tt.target)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, oczekiwano %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q", rec.Body.String())
			}
			if tt.wantCache != "" && rec.Header().Get("Cache-Control") != tt.wantCache {
				t.Errorf("Cache-Control = %q", rec.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestSPAWithoutBuiltFrontend(t *testing.T) {
	h := newTestRouter(fakeDB{}, fstest.MapFS{".gitkeep": {}})
	rec := do(t, h, http.MethodGet, "/")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "npm run build") {
		t.Errorf("brak instrukcji w treści: %q", rec.Body.String())
	}
	// API działa nawet bez frontendu.
	if rec := do(t, h, http.MethodGet, "/api/health"); rec.Code != http.StatusOK {
		t.Errorf("health status = %d", rec.Code)
	}
}
