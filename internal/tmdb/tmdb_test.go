package tmdb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeTMDB(t *testing.T, check func(r *http.Request)) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /search/multi", func(w http.ResponseWriter, r *http.Request) {
		check(r)
		if r.URL.Query().Get("query") != "diuna" || r.URL.Query().Get("include_adult") != "false" {
			t.Errorf("zapytanie = %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"results": [
		  {"id": 693134, "media_type": "movie", "title": "Diuna: Część druga", "release_date": "2024-02-27",
		   "poster_path": "/abc.jpg", "vote_average": 8.16, "vote_count": 5000, "genre_ids": [878, 12]},
		  {"id": 90228, "media_type": "tv", "name": "Diuna: Proroctwo", "first_air_date": "2024-11-17",
		   "poster_path": null, "vote_average": 0, "vote_count": 0, "genre_ids": [10765]},
		  {"id": 1, "media_type": "person", "name": "Denis Villeneuve"}
		]}`)
	})
	mux.HandleFunc("GET /genre/movie/list", func(w http.ResponseWriter, r *http.Request) {
		check(r)
		_, _ = io.WriteString(w, `{"genres": [{"id": 878, "name": "Sci-Fi"}, {"id": 12, "name": "Przygodowy"}]}`)
	})
	mux.HandleFunc("GET /genre/tv/list", func(w http.ResponseWriter, r *http.Request) {
		check(r)
		_, _ = io.WriteString(w, `{"genres": [{"id": 10765, "name": "Sci-Fi & Fantasy"}]}`)
	})
	mux.HandleFunc("GET /movie/693134", func(w http.ResponseWriter, r *http.Request) {
		check(r)
		if r.URL.Query().Get("append_to_response") != "credits" {
			t.Error("brak credits")
		}
		_, _ = io.WriteString(w, `{"id": 693134, "title": "Diuna: Część druga", "release_date": "2024-02-27",
		  "poster_path": "/abc.jpg", "vote_average": 8.16, "vote_count": 5000, "runtime": 166,
		  "genres": [{"id": 878, "name": "Sci-Fi"}],
		  "credits": {"crew": [{"name": "Hans Zimmer", "job": "Original Music Composer"}, {"name": "Denis Villeneuve", "job": "Director"}]}}`)
	})
	mux.HandleFunc("GET /tv/90228", func(w http.ResponseWriter, r *http.Request) {
		check(r)
		_, _ = io.WriteString(w, `{"id": 90228, "name": "Diuna: Proroctwo", "first_air_date": "2024-11-17",
		  "number_of_episodes": 6, "created_by": [{"name": "Diane Ademu-John"}], "genres": []}`)
	})
	mux.HandleFunc("GET /movie/404", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSearchAndDetails(t *testing.T) {
	srv := fakeTMDB(t, func(r *http.Request) {
		if r.URL.Query().Get("api_key") != "k3y" || r.URL.Query().Get("language") != "pl-PL" {
			t.Errorf("%s: zapytanie bez klucza albo języka: %s", r.URL.Path, r.URL.RawQuery)
		}
	})
	c := New("k3y", WithBaseURL(srv.URL))
	ctx := context.Background()

	got, err := c.Search(ctx, "diuna")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("wyników = %d (osoby mają być pominięte)", len(got))
	}
	m := got[0]
	if m.Kind != Movie || m.Title != "Diuna: Część druga" || *m.Year != 2024 || *m.Score != 82 ||
		m.PosterURL != "https://image.tmdb.org/t/p/w342/abc.jpg" || strings.Join(m.Genres, ",") != "Sci-Fi,Przygodowy" {
		t.Errorf("film = %+v", m)
	}
	s := got[1]
	if s.Kind != TV || s.Title != "Diuna: Proroctwo" || s.Score != nil || s.PosterURL != "" || s.Genres[0] != "Sci-Fi & Fantasy" {
		t.Errorf("serial = %+v", s)
	}

	d, err := c.Details(ctx, Movie, 693134)
	if err != nil || d.Creator != "Denis Villeneuve" || *d.RuntimeMin != 166 || d.Episodes != nil {
		t.Errorf("szczegóły filmu = %+v, %v", d, err)
	}
	d, err = c.Details(ctx, TV, 90228)
	if err != nil || d.Creator != "Diane Ademu-John" || *d.Episodes != 6 || d.RuntimeMin != nil {
		t.Errorf("szczegóły serialu = %+v, %v", d, err)
	}
	if _, err := c.Details(ctx, Movie, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("404: err = %v", err)
	}
}

func TestBearerToken(t *testing.T) {
	token := "eyJhbGciOiJIUzI1NiJ9." + strings.Repeat("a", 80) + ".sig"
	srv := fakeTMDB(t, func(r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.Query().Has("api_key") {
			t.Errorf("token v4 nie poszedł w nagłówku: %q %s", r.Header.Get("Authorization"), r.URL.RawQuery)
		}
	})
	if _, err := New(token, WithBaseURL(srv.URL)).Search(context.Background(), "diuna"); err != nil {
		t.Fatal(err)
	}
}

func TestErrors(t *testing.T) {
	for _, tt := range []struct {
		code int
		want error
	}{{401, ErrBadKey}, {500, ErrUnavailable}, {429, ErrUnavailable}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tt.code) }))
		_, err := New("k", WithBaseURL(srv.URL)).Search(context.Background(), "x")
		srv.Close()
		if !errors.Is(err, tt.want) {
			t.Errorf("%d: err = %v", tt.code, err)
		}
	}
}
