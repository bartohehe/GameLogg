// Package config wczytuje konfigurację aplikacji wyłącznie ze zmiennych
// środowiskowych i waliduje ją przy starcie.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// MinJWTSecretLen to minimalna długość JWT_SECRET przy włączonym logowaniu.
const MinJWTSecretLen = 32

// Config to pełna konfiguracja aplikacji.
type Config struct {
	// Dev przełącza logi na czytelny tekst z poziomem debug (APP_ENV=dev).
	Dev bool

	Port        int
	DatabaseURL string

	AuthEnabled bool
	JWTSecret   string

	IGDBClientID     string
	IGDBClientSecret string
	TMDBAPIKey       string

	MediaEnabled       bool
	TranslationEnabled bool
}

// IGDBConfigured mówi, czy podano oba klucze IGDB.
func (c Config) IGDBConfigured() bool {
	return c.IGDBClientID != "" && c.IGDBClientSecret != ""
}

// TMDBConfigured mówi, czy podano klucz TMDB.
func (c Config) TMDBConfigured() bool {
	return c.TMDBAPIKey != ""
}

// Addr to adres nasłuchu serwera HTTP.
func (c Config) Addr() string {
	return ":" + strconv.Itoa(c.Port)
}

// Load wczytuje konfigurację przez lookup (zwykle os.LookupEnv) i waliduje ją.
// Zwraca wszystkie znalezione błędy naraz, żeby dało się poprawić .env za jednym razem.
func Load(lookup func(string) (string, bool)) (Config, error) {
	get := func(key string) string {
		v, _ := lookup(key)
		return strings.TrimSpace(v)
	}

	var errs []error

	// Docker Compose przy pustej wartości z komentarzem w tej samej linii
	// (`JWT_SECRET=   # opis`) przekazuje komentarz jako wartość. Taki "sekret"
	// przeszedłby walidację długości, więc odrzucamy go jawnie.
	for _, key := range []string{
		"DATABASE_URL", "JWT_SECRET", "IGDB_CLIENT_ID", "IGDB_CLIENT_SECRET", "TMDB_API_KEY",
		"AUTH_ENABLED", "MEDIA_ENABLED", "TRANSLATION_ENABLED", "PORT", "APP_ENV",
	} {
		if strings.HasPrefix(get(key), "#") {
			errs = append(errs, fmt.Errorf(
				"%s zaczyna się od #, czyli to prawdopodobnie komentarz z .env wczytany jako wartość: przenieś komentarz do osobnej linii",
				key))
		}
	}

	boolVar := func(key string, def bool) bool {
		raw := get(key)
		if raw == "" {
			return def
		}
		v, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s ma wartość %q, a dozwolone są true albo false", key, raw))
			return def
		}
		return v
	}

	cfg := Config{
		DatabaseURL:        get("DATABASE_URL"),
		JWTSecret:          get("JWT_SECRET"),
		IGDBClientID:       get("IGDB_CLIENT_ID"),
		IGDBClientSecret:   get("IGDB_CLIENT_SECRET"),
		TMDBAPIKey:         get("TMDB_API_KEY"),
		AuthEnabled:        boolVar("AUTH_ENABLED", true),
		MediaEnabled:       boolVar("MEDIA_ENABLED", true),
		TranslationEnabled: boolVar("TRANSLATION_ENABLED", true),
		Port:               8080,
	}

	switch env := strings.ToLower(get("APP_ENV")); env {
	case "", "production", "prod":
	case "dev", "development":
		cfg.Dev = true
	default:
		errs = append(errs, fmt.Errorf("APP_ENV ma wartość %q, a dozwolone są production albo dev", env))
	}

	if raw := get("PORT"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("PORT ma wartość %q, a musi być liczbą od 1 do 65535", raw))
		} else {
			cfg.Port = p
		}
	}

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("brak DATABASE_URL"))
	} else if u, err := url.Parse(cfg.DatabaseURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		errs = append(errs, errors.New("DATABASE_URL musi zaczynać się od postgres:// albo postgresql://"))
	}

	if cfg.AuthEnabled && len(cfg.JWTSecret) < MinJWTSecretLen {
		errs = append(errs, fmt.Errorf(
			"JWT_SECRET musi mieć co najmniej %d znaków, gdy AUTH_ENABLED=true (wygeneruj np. przez: openssl rand -hex 32)",
			MinJWTSecretLen))
	}

	if (cfg.IGDBClientID == "") != (cfg.IGDBClientSecret == "") {
		errs = append(errs, errors.New("IGDB_CLIENT_ID i IGDB_CLIENT_SECRET trzeba podać razem albo oba zostawić puste"))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("błędna konfiguracja:\n%w", errors.Join(errs...))
	}
	return cfg, nil
}
