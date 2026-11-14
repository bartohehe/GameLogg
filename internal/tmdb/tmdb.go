// Package tmdb to klient TMDB do wyszukiwania filmów i seriali (po polsku).
package tmdb

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
)

const (
	defaultBaseURL = "https://api.themoviedb.org/3"
	posterBase     = "https://image.tmdb.org/t/p/w342"
	language       = "pl-PL"
)

var (
	ErrNotFound    = errors.New("tmdb: tytuł nie istnieje")
	ErrBadKey      = errors.New("tmdb: nieprawidłowy klucz API")
	ErrUnavailable = errors.New("tmdb: usługa niedostępna")
)

// Kind to rodzaj tytułu w TMDB.
type Kind string

const (
	Movie Kind = "movie"
	TV    Kind = "tv"
)

// Title to film albo serial.
type Title struct {
	ID         int64
	Kind       Kind
	Title      string
	Year       *int
	PosterURL  string
	Score      *int // vote_average × 10, nil bez głosów
	Genres     []string
	Creator    string // reżyser filmu albo twórcy serialu (tylko w Details)
	RuntimeMin *int   // filmy (tylko w Details)
	Episodes   *int   // seriale (tylko w Details)
	Overview   string
}

// Client rozmawia z TMDB. Bezpieczny do współbieżnego użycia.
type Client struct {
	key     string
	bearer  bool
	http    *http.Client
	baseURL string

	genreMu sync.Mutex
	genres  map[int]string
}

// Option zmienia ustawienia (testy).
type Option func(*Client)

// WithBaseURL podmienia adres API.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") } }

// New tworzy klienta. Przyjmuje klucz API v3 (32 znaki) albo token v4
// (JWT), który idzie w nagłówku Authorization.
func New(key string, opts ...Option) *Client {
	c := &Client{
		key:     key,
		bearer:  strings.Count(key, ".") == 2 && len(key) > 64,
		http:    &http.Client{Timeout: 8 * time.Second},
		baseURL: defaultBaseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("language", language)
	if !c.bearer {
		params.Set("api_key", c.key)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.bearer {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrBadKey
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

type rawResult struct {
	ID           int64   `json:"id"`
	MediaType    string  `json:"media_type"`
	Title        string  `json:"title"` // filmy
	Name         string  `json:"name"`  // seriale
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	GenreIDs     []int   `json:"genre_ids"`
	Overview     string  `json:"overview"`
}

func yearOf(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return nil
	}
	return &y
}

func score(avg float64, count int) *int {
	if count == 0 {
		return nil
	}
	s := int(math.Round(avg * 10))
	s = max(0, min(100, s))
	return &s
}

func poster(path string) string {
	if path == "" {
		return ""
	}
	return posterBase + path
}

func (r rawResult) toTitle(kind Kind, genres map[int]string) Title {
	t := Title{ID: r.ID, Kind: kind, PosterURL: poster(r.PosterPath), Score: score(r.VoteAverage, r.VoteCount),
		Overview: r.Overview, Genres: []string{}}
	if kind == Movie {
		t.Title, t.Year = r.Title, yearOf(r.ReleaseDate)
	} else {
		t.Title, t.Year = r.Name, yearOf(r.FirstAirDate)
	}
	for _, id := range r.GenreIDs {
		if g, ok := genres[id]; ok {
			t.Genres = append(t.Genres, g)
		}
	}
	return t
}

// Search szuka filmów i seriali (search/multi, po polsku). Osoby pomija.
func (c *Client) Search(ctx context.Context, q string) ([]Title, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, nil
	}
	var resp struct {
		Results []rawResult `json:"results"`
	}
	params := url.Values{"query": {q}, "include_adult": {"false"}, "page": {"1"}}
	if err := c.get(ctx, "/search/multi", params, &resp); err != nil {
		return nil, err
	}
	genres := c.genreNames(ctx)
	out := []Title{}
	for _, r := range resp.Results {
		switch Kind(r.MediaType) {
		case Movie, TV:
			out = append(out, r.toTitle(Kind(r.MediaType), genres))
		}
	}
	return out, nil
}

// genreNames pobiera raz polskie nazwy gatunków (filmy i seriale). Błąd nie
// psuje wyszukiwania: wyniki są wtedy bez gatunków, a lista pobierze się później.
func (c *Client) genreNames(ctx context.Context) map[int]string {
	c.genreMu.Lock()
	defer c.genreMu.Unlock()
	if c.genres != nil {
		return c.genres
	}
	names := map[int]string{}
	for _, kind := range []Kind{Movie, TV} {
		var resp struct {
			Genres []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"genres"`
		}
		if err := c.get(ctx, "/genre/"+string(kind)+"/list", nil, &resp); err != nil {
			return names
		}
		for _, g := range resp.Genres {
			names[g.ID] = g.Name
		}
	}
	c.genres = names
	return names
}

// Details zwraca pełne dane tytułu: reżysera albo twórców, czas trwania filmu
// albo liczbę odcinków serialu.
func (c *Client) Details(ctx context.Context, kind Kind, id int64) (Title, error) {
	var r struct {
		rawResult
		Genres []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Runtime          *int `json:"runtime"`
		NumberOfEpisodes *int `json:"number_of_episodes"`
		CreatedBy        []struct {
			Name string `json:"name"`
		} `json:"created_by"`
		Credits struct {
			Crew []struct {
				Name string `json:"name"`
				Job  string `json:"job"`
			} `json:"crew"`
		} `json:"credits"`
	}
	var params url.Values
	switch kind {
	case Movie:
		params = url.Values{"append_to_response": {"credits"}}
	case TV:
	default:
		return Title{}, ErrNotFound
	}
	if err := c.get(ctx, "/"+string(kind)+"/"+strconv.FormatInt(id, 10), params, &r); err != nil {
		return Title{}, err
	}

	t := r.rawResult.toTitle(kind, nil)
	for _, g := range r.Genres {
		t.Genres = append(t.Genres, g.Name)
	}
	var creators []string
	if kind == Movie {
		for _, p := range r.Credits.Crew {
			if p.Job == "Director" && len(creators) < 2 {
				creators = append(creators, p.Name)
			}
		}
		if r.Runtime != nil && *r.Runtime > 0 {
			t.RuntimeMin = r.Runtime
		}
	} else {
		for _, p := range r.CreatedBy {
			if len(creators) < 2 {
				creators = append(creators, p.Name)
			}
		}
		if r.NumberOfEpisodes != nil && *r.NumberOfEpisodes > 0 {
			t.Episodes = r.NumberOfEpisodes
		}
	}
	t.Creator = strings.Join(creators, ", ")
	return t, nil
}
