package jikan

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const frieren = `{
  "mal_id": 52991, "title": "Sousou no Frieren", "title_english": "Frieren: Beyond Journey's End",
  "type": "TV", "episodes": 28, "duration": "24 min per ep", "score": 9.31, "year": 2023,
  "images": {"jpg": {"image_url": "https://cdn.myanimelist.net/s.jpg", "large_image_url": "https://cdn.myanimelist.net/l.jpg"}},
  "studios": [{"name": "Madhouse"}], "genres": [{"name": "Adventure"}, {"name": "Drama"}]
}`

func TestSearchAndAnime(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /anime", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "frieren" || q.Get("limit") != "20" || q.Get("sfw") != "true" {
			t.Errorf("zapytanie = %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data": [`+frieren+`, {"mal_id": 2, "title": "Look Back", "type": "Movie",
		  "duration": "58 min", "episodes": 1, "score": null, "year": null, "aired": {"prop": {"from": {"year": 2024}}},
		  "images": {"jpg": {"image_url": "https://cdn/x.jpg"}}, "studios": [], "genres": []}]}`)
	})
	mux.HandleFunc("GET /anime/52991", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"data": `+frieren+`}`)
	})
	mux.HandleFunc("GET /anime/1", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(WithBaseURL(srv.URL))
	ctx := context.Background()
	got, err := c.Search(ctx, "frieren")
	if err != nil || len(got) != 2 {
		t.Fatalf("Search = %+v, %v", got, err)
	}
	f := got[0]
	if f.Title != "Frieren: Beyond Journey's End" || *f.Year != 2023 || *f.Score != 93 || *f.Episodes != 28 ||
		f.Studios != "Madhouse" || f.ImageURL != "https://cdn.myanimelist.net/l.jpg" || f.RuntimeMin != nil || len(f.Genres) != 2 {
		t.Errorf("Frieren = %+v", f)
	}
	lb := got[1]
	if lb.Title != "Look Back" || *lb.Year != 2024 || lb.Score != nil || *lb.RuntimeMin != 58 || lb.ImageURL != "https://cdn/x.jpg" {
		t.Errorf("Look Back = %+v", lb)
	}

	a, err := c.Anime(ctx, 52991)
	if err != nil || a.ID != 52991 {
		t.Errorf("Anime = %+v, %v", a, err)
	}
	if _, err := c.Anime(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: err = %v", err)
	}
}

func TestThrottleSpacesRequests(t *testing.T) {
	var mu sync.Mutex
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	var waits []time.Duration
	c := New()
	c.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	c.wait = func(_ context.Context, d time.Duration) error {
		mu.Lock()
		defer mu.Unlock()
		waits = append(waits, d)
		return nil
	}
	for range 4 {
		if err := c.throttle(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	// Pierwsze od razu, kolejne co 340 ms: najwyżej 3 żądania na sekundę.
	want := []time.Duration{MinInterval, 2 * MinInterval, 3 * MinInterval}
	if len(waits) != 3 {
		t.Fatalf("czekań = %v", waits)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Errorf("czekanie %d = %v, oczekiwano %v", i, waits[i], want[i])
		}
	}

	// Po przerwie dłuższej niż odstęp nie ma czekania.
	mu.Lock()
	now = now.Add(5 * time.Second)
	mu.Unlock()
	waits = nil
	_ = c.throttle(context.Background())
	if len(waits) != 0 {
		t.Errorf("po przerwie czekanie %v", waits)
	}
}

func TestThrottleRespectsContext(t *testing.T) {
	c := New()
	_ = c.throttle(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.throttle(ctx); err == nil {
		t.Error("anulowany kontekst powinien przerwać czekanie")
	}
}

func TestParseDuration(t *testing.T) {
	tests := map[string]int{"24 min per ep": 24, "2 hr 10 min": 130, "1 hr": 60, "58 min": 58}
	for in, want := range tests {
		if got := ParseDuration(in); got == nil || *got != want {
			t.Errorf("ParseDuration(%q) = %v, oczekiwano %d", in, got, want)
		}
	}
	for _, in := range []string{"Unknown", ""} {
		if got := ParseDuration(in); got != nil {
			t.Errorf("ParseDuration(%q) = %d", in, *got)
		}
	}
}

func TestUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(429) }))
	defer srv.Close()
	if _, err := New(WithBaseURL(srv.URL)).Search(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("429: err = %v", err)
	}
}
