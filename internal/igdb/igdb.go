// Package igdb to klient IGDB z tokenem aplikacji Twitch (client credentials)
// trzymanym w pamięci i odnawianym przed wygaśnięciem.
package igdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultTokenURL = "https://id.twitch.tv/oauth2/token"
	defaultAPIURL   = "https://api.igdb.com/v4"
	coverURLFormat  = "https://images.igdb.com/igdb/image/upload/t_cover_big/%s.jpg"

	// Token odnawiamy z takim zapasem przed wygaśnięciem.
	tokenRefreshMargin = 5 * time.Minute
	maxQueryRunes      = 100
	maxResponseBytes   = 4 << 20
)

var (
	// ErrNotFound: IGDB nie zna gry o tym identyfikatorze.
	ErrNotFound = errors.New("igdb: gra nie istnieje")
	// ErrBadCredentials: Twitch odrzucił IGDB_CLIENT_ID albo IGDB_CLIENT_SECRET.
	ErrBadCredentials = errors.New("igdb: nieprawidłowe klucze")
	// ErrUnavailable: IGDB albo Twitch nie odpowiada, zwraca 5xx albo limituje.
	ErrUnavailable = errors.New("igdb: usługa niedostępna")
)

// Game to gra w kształcie używanym przez aplikację.
type Game struct {
	ID          int64
	Title       string
	CoverURL    string     // pusty, gdy IGDB nie ma okładki
	ReleaseDate *time.Time // nil, gdy brak daty
	Developer   string
	Genres      []string
	Platforms   []string // etykiety platform, np. "PC", "PS5", "Xbox Series", "Switch"
	Rating      *int     // total_rating zaokrąglony do 0–100
	Summary     string   // po angielsku, tak jak w IGDB
}

// Client rozmawia z IGDB. Bezpieczny do współbieżnego użycia.
type Client struct {
	clientID string
	secret   string
	http     *http.Client
	tokenURL string
	apiURL   string
	now      func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time

	popMu          sync.Mutex
	popular        []Game
	popularExpires time.Time
}

// Option zmienia ustawienia klienta (używane w testach).
type Option func(*Client)

// WithEndpoints podmienia adresy Twitch i IGDB.
func WithEndpoints(tokenURL, apiURL string) Option {
	return func(c *Client) { c.tokenURL, c.apiURL = tokenURL, strings.TrimRight(apiURL, "/") }
}

// WithClock podmienia zegar.
func WithClock(now func() time.Time) Option {
	return func(c *Client) { c.now = now }
}

// New tworzy klienta. Klucze muszą być niepuste (sprawdza to config).
func New(clientID, secret string, opts ...Option) *Client {
	c := &Client{
		clientID: clientID,
		secret:   secret,
		http:     &http.Client{Timeout: 10 * time.Second},
		tokenURL: defaultTokenURL,
		apiURL:   defaultAPIURL,
		now:      time.Now,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

const gameFields = "fields id,name,cover.image_id,first_release_date,genres.name," +
	"platforms.abbreviation,platforms.name,involved_companies.company.name," +
	"involved_companies.developer,total_rating"

// Search szuka gier po tytule (najwyżej 12 wyników).
func (c *Client) Search(ctx context.Context, q string) ([]Game, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	body := fmt.Sprintf("search %s; %s; limit 12;", Quote(q), gameFields)
	return c.games(ctx, body)
}

// Game zwraca szczegóły jednej gry razem z opisem.
func (c *Client) Game(ctx context.Context, id int64) (Game, error) {
	body := fmt.Sprintf("%s,summary; where id = %d; limit 1;", gameFields, id)
	gs, err := c.games(ctx, body)
	if err != nil {
		return Game{}, err
	}
	if len(gs) == 0 {
		return Game{}, ErrNotFound
	}
	return gs[0], nil
}

// Quote zamienia tekst użytkownika na bezpieczny literał napisu apicalypse:
// usuwa znaki sterujące, przycina długość i escapuje \ oraz ".
func Quote(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	if r := []rune(s); len(r) > maxQueryRunes {
		s = string(r[:maxQueryRunes])
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

func (c *Client) games(ctx context.Context, body string) ([]Game, error) {
	var raw []rawGame
	if err := c.query(ctx, "games", body, &raw); err != nil {
		return nil, err
	}
	out := make([]Game, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toGame())
	}
	return out, nil
}

// query wysyła zapytanie apicalypse. Przy 401 odświeża token i próbuje raz jeszcze.
func (c *Client) query(ctx context.Context, endpoint, body string, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.accessToken(ctx, attempt > 0)
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/"+endpoint, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Client-ID", c.clientID)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "text/plain")

		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrUnavailable, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()

		switch {
		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			continue // token mógł zostać unieważniony, pobierz nowy
		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			return ErrBadCredentials
		case resp.StatusCode != http.StatusOK:
			return fmt.Errorf("%w: status %d: %s", ErrUnavailable, resp.StatusCode, truncate(data))
		case readErr != nil:
			return fmt.Errorf("%w: %v", ErrUnavailable, readErr)
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%w: niepoprawna odpowiedź: %v", ErrUnavailable, err)
		}
		return nil
	}
	return ErrBadCredentials
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (c *Client) accessToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && c.now().Before(c.expires.Add(-tokenRefreshMargin)) {
		return c.token, nil
	}

	form := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.secret},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	switch {
	case resp.StatusCode == http.StatusBadRequest, resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return "", ErrBadCredentials
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("%w: token: status %d", ErrUnavailable, resp.StatusCode)
	}
	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil || tr.AccessToken == "" {
		return "", fmt.Errorf("%w: token: niepoprawna odpowiedź", ErrUnavailable)
	}
	c.token = tr.AccessToken
	c.expires = c.now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return c.token, nil
}

func truncate(b []byte) string {
	if len(b) > 200 {
		b = b[:200]
	}
	return string(b)
}

type rawGame struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Cover *struct {
		ImageID string `json:"image_id"`
	} `json:"cover"`
	FirstReleaseDate *int64 `json:"first_release_date"`
	Genres           []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Platforms []struct {
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"name"`
	} `json:"platforms"`
	InvolvedCompanies []struct {
		Company struct {
			Name string `json:"name"`
		} `json:"company"`
		Developer bool `json:"developer"`
	} `json:"involved_companies"`
	TotalRating *float64 `json:"total_rating"`
	Summary     string   `json:"summary"`
}

func (r rawGame) toGame() Game {
	g := Game{ID: r.ID, Title: r.Name, Summary: r.Summary, Genres: []string{}, Platforms: []string{}}
	if r.Cover != nil && r.Cover.ImageID != "" {
		g.CoverURL = fmt.Sprintf(coverURLFormat, url.PathEscape(r.Cover.ImageID))
	}
	if r.FirstReleaseDate != nil {
		t := time.Unix(*r.FirstReleaseDate, 0).UTC()
		g.ReleaseDate = &t
	}
	for _, gen := range r.Genres {
		if gen.Name != "" {
			g.Genres = append(g.Genres, gen.Name)
		}
	}
	var devs []string
	for _, ic := range r.InvolvedCompanies {
		if ic.Developer && ic.Company.Name != "" && len(devs) < 2 {
			devs = append(devs, ic.Company.Name)
		}
	}
	g.Developer = strings.Join(devs, ", ")
	labels := make([]string, 0, len(r.Platforms))
	for _, p := range r.Platforms {
		labels = append(labels, PlatformLabel(p.Abbreviation, p.Name))
	}
	g.Platforms = SortPlatforms(labels)
	if r.TotalRating != nil {
		v := int(math.Round(*r.TotalRating))
		v = max(0, min(100, v))
		g.Rating = &v
	}
	return g
}

// ParseID czyta identyfikator gry IGDB z parametru ścieżki.
func ParseID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id > 0
}
