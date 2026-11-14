// Package jikan to klient Jikan (nieoficjalne API MyAnimeList) dla anime.
// Jikan pozwala na 3 żądania na sekundę, więc klient sam je rozkłada w czasie.
package jikan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBaseURL = "https://api.jikan.moe/v4"

// MinInterval to odstęp między żądaniami: 3 na sekundę z małym zapasem.
const MinInterval = 340 * time.Millisecond

var (
	ErrNotFound    = errors.New("jikan: anime nie istnieje")
	ErrUnavailable = errors.New("jikan: usługa niedostępna")
)

// Anime to tytuł z MyAnimeList.
type Anime struct {
	ID         int64
	Title      string
	Year       *int
	ImageURL   string
	Score      *int // score × 10
	Genres     []string
	Studios    string
	Episodes   *int
	RuntimeMin *int // dla filmów anime (z pola duration)
	Type       string
}

// Client rozmawia z Jikan. Bezpieczny do współbieżnego użycia.
type Client struct {
	http    *http.Client
	baseURL string

	mu   sync.Mutex
	last time.Time
	now  func() time.Time
	wait func(ctx context.Context, d time.Duration) error
}

// Option zmienia ustawienia (testy).
type Option func(*Client)

// WithBaseURL podmienia adres API.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// New tworzy klienta.
func New(opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{Timeout: 8 * time.Second},
		baseURL: defaultBaseURL,
		now:     time.Now,
		wait: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// throttle czeka, aż od poprzedniego żądania minie MinInterval. Kolejne żądania
// rezerwują swoje okna po kolei, więc nigdy nie wychodzi więcej niż 3 na sekundę.
func (c *Client) throttle(ctx context.Context) error {
	c.mu.Lock()
	now := c.now()
	slot := c.last.Add(MinInterval)
	if slot.Before(now) {
		slot = now
	}
	c.last = slot
	c.mu.Unlock()
	if d := slot.Sub(now); d > 0 {
		return c.wait(ctx, d)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	if err := c.throttle(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: niepoprawna odpowiedź", ErrUnavailable)
	}
	return nil
}

type rawAnime struct {
	MalID        int64  `json:"mal_id"`
	Title        string `json:"title"`
	TitleEnglish string `json:"title_english"`
	Type         string `json:"type"`
	Images       struct {
		JPG struct {
			ImageURL      string `json:"image_url"`
			LargeImageURL string `json:"large_image_url"`
		} `json:"jpg"`
	} `json:"images"`
	Year  *int `json:"year"`
	Aired struct {
		Prop struct {
			From struct {
				Year *int `json:"year"`
			} `json:"from"`
		} `json:"prop"`
	} `json:"aired"`
	Score    *float64 `json:"score"`
	Episodes *int     `json:"episodes"`
	Duration string   `json:"duration"`
	Studios  []struct {
		Name string `json:"name"`
	} `json:"studios"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
}

var durationRe = regexp.MustCompile(`(?:(\d+)\s*hr)?\s*(?:(\d+)\s*min)?`)

// ParseDuration zamienia "2 hr 10 min" albo "24 min per ep" na minuty.
func ParseDuration(s string) *int {
	m := durationRe.FindStringSubmatch(s)
	if m == nil || (m[1] == "" && m[2] == "") {
		return nil
	}
	h, _ := strconv.Atoi(m[1])
	mins, _ := strconv.Atoi(m[2])
	total := h*60 + mins
	if total <= 0 {
		return nil
	}
	return &total
}

func (r rawAnime) toAnime() Anime {
	a := Anime{ID: r.MalID, Title: r.Title, Type: r.Type, Genres: []string{}}
	if r.TitleEnglish != "" {
		a.Title = r.TitleEnglish
	}
	a.ImageURL = r.Images.JPG.LargeImageURL
	if a.ImageURL == "" {
		a.ImageURL = r.Images.JPG.ImageURL
	}
	a.Year = r.Year
	if a.Year == nil {
		a.Year = r.Aired.Prop.From.Year
	}
	if r.Score != nil {
		s := max(0, min(100, int(math.Round(*r.Score*10))))
		a.Score = &s
	}
	if r.Episodes != nil && *r.Episodes > 0 {
		a.Episodes = r.Episodes
	}
	if r.Type == "Movie" {
		a.RuntimeMin = ParseDuration(r.Duration)
	}
	var studios []string
	for _, s := range r.Studios {
		if len(studios) < 2 {
			studios = append(studios, s.Name)
		}
	}
	a.Studios = strings.Join(studios, ", ")
	for _, g := range r.Genres {
		a.Genres = append(a.Genres, g.Name)
	}
	return a
}

// Search szuka anime (tylko bezpieczne dla pracy, do 20 wyników).
func (c *Client) Search(ctx context.Context, q string) ([]Anime, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	var resp struct {
		Data []rawAnime `json:"data"`
	}
	if err := c.get(ctx, "/anime", url.Values{"q": {q}, "limit": {"20"}, "sfw": {"true"}}, &resp); err != nil {
		return nil, err
	}
	out := make([]Anime, 0, len(resp.Data))
	for _, r := range resp.Data {
		out = append(out, r.toAnime())
	}
	return out, nil
}

// Anime zwraca jeden tytuł.
func (c *Client) Anime(ctx context.Context, id int64) (Anime, error) {
	var resp struct {
		Data rawAnime `json:"data"`
	}
	if err := c.get(ctx, "/anime/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return Anime{}, err
	}
	return resp.Data.toAnime(), nil
}
