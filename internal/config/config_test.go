package config

import (
	"strings"
	"testing"
)

const validSecret = "0123456789abcdef0123456789abcdef"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) {
		v, ok := env[k]
		return v, ok
	}
}

func baseEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://gamelogg:pw@localhost:5432/gamelogg?sslmode=disable",
		"JWT_SECRET":   validSecret,
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookupFrom(baseEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 || cfg.Addr() != ":8080" {
		t.Errorf("port = %d, addr = %q", cfg.Port, cfg.Addr())
	}
	if !cfg.AuthEnabled || !cfg.MediaEnabled || !cfg.TranslationEnabled {
		t.Errorf("flagi domyślnie mają być włączone: %+v", cfg)
	}
	if cfg.Dev {
		t.Error("domyślnie tryb produkcyjny")
	}
	if cfg.IGDBConfigured() || cfg.TMDBConfigured() {
		t.Error("bez kluczy integracje nie są skonfigurowane")
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string // pusty = brak błędu
		check   func(t *testing.T, c Config)
	}{
		{
			name:    "brak DATABASE_URL",
			env:     map[string]string{"DATABASE_URL": "", "JWT_SECRET": validSecret},
			wantErr: "brak DATABASE_URL",
		},
		{
			name:    "zły schemat DATABASE_URL",
			env:     map[string]string{"DATABASE_URL": "mysql://x", "JWT_SECRET": validSecret},
			wantErr: "postgres://",
		},
		{
			name:    "krótki JWT_SECRET przy auth",
			env:     map[string]string{"JWT_SECRET": "short"},
			wantErr: "JWT_SECRET",
		},
		{
			name: "brak JWT_SECRET bez auth jest ok",
			env:  map[string]string{"JWT_SECRET": "", "AUTH_ENABLED": "false"},
			check: func(t *testing.T, c Config) {
				if c.AuthEnabled {
					t.Error("AuthEnabled powinno być false")
				}
			},
		},
		{
			name:    "zły bool",
			env:     map[string]string{"MEDIA_ENABLED": "tak"},
			wantErr: "MEDIA_ENABLED",
		},
		{
			name:    "zły port",
			env:     map[string]string{"PORT": "70000"},
			wantErr: "PORT",
		},
		{
			name: "własny port",
			env:  map[string]string{"PORT": " 9000 "},
			check: func(t *testing.T, c Config) {
				if c.Port != 9000 {
					t.Errorf("port = %d", c.Port)
				}
			},
		},
		{
			name:    "tylko jeden klucz IGDB",
			env:     map[string]string{"IGDB_CLIENT_ID": "abc"},
			wantErr: "IGDB_CLIENT_ID i IGDB_CLIENT_SECRET",
		},
		{
			name: "oba klucze IGDB i TMDB",
			env:  map[string]string{"IGDB_CLIENT_ID": "abc", "IGDB_CLIENT_SECRET": "def", "TMDB_API_KEY": "k"},
			check: func(t *testing.T, c Config) {
				if !c.IGDBConfigured() || !c.TMDBConfigured() {
					t.Error("integracje powinny być skonfigurowane")
				}
			},
		},
		{
			name: "APP_ENV=dev",
			env:  map[string]string{"APP_ENV": "dev"},
			check: func(t *testing.T, c Config) {
				if !c.Dev {
					t.Error("Dev powinno być true")
				}
			},
		},
		{
			// Tak Docker Compose wczytuje `JWT_SECRET=      # min. 32 znaki, ...`.
			name:    "komentarz z .env jako sekret",
			env:     map[string]string{"JWT_SECRET": "# min. 32 znaki, start bez niego = błąd, gdy AUTH_ENABLED=true"},
			wantErr: "JWT_SECRET zaczyna się od #",
		},
		{
			name:    "komentarz z .env jako klucz TMDB",
			env:     map[string]string{"TMDB_API_KEY": "# opcjonalny"},
			wantErr: "TMDB_API_KEY zaczyna się od #",
		},
		{
			name:    "nieznane APP_ENV",
			env:     map[string]string{"APP_ENV": "staging"},
			wantErr: "APP_ENV",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := baseEnv()
			for k, v := range tt.env {
				env[k] = v
			}
			cfg, err := Load(lookupFrom(env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("błąd = %v, oczekiwano zawierającego %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := Load(lookupFrom(map[string]string{"PORT": "abc", "AUTH_ENABLED": "true"}))
	if err == nil {
		t.Fatal("oczekiwano błędu")
	}
	for _, want := range []string{"DATABASE_URL", "JWT_SECRET", "PORT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("błąd nie wspomina %s: %v", want, err)
		}
	}
}
