package igdb

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{`persona`, `"persona"`},
		{`say "hi"`, `"say \"hi\""`},
		{`back\slash`, `"back\\slash"`},
		{`"; fields *; where id > 0; "`, `"\"; fields *; where id > 0; \""`},
		{"nowa\nlinia\t", `"nowa linia "`},
		{`\"`, `"\\\""`},
		{`Zażółć gęślą`, `"Zażółć gęślą"`},
	}
	for _, tt := range tests {
		if got := Quote(tt.in); got != tt.want {
			t.Errorf("Quote(%q) = %s, oczekiwano %s", tt.in, got, tt.want)
		}
	}
	if got := Quote(strings.Repeat("a", 300)); len(got) != maxQueryRunes+2 {
		t.Errorf("długość po przycięciu = %d", len(got))
	}
}

// fakeIGDB udaje Twitch i IGDB.
type fakeIGDB struct {
	tokenCalls atomic.Int32
	apiCalls   atomic.Int32
	lastBody   atomic.Value
	tokenCode  int
	apiStatus  func(call int32) int
	response   string
}

func (f *fakeIGDB) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		n := f.tokenCalls.Add(1)
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" || r.Form.Get("client_id") != "id" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if f.tokenCode != 0 {
			w.WriteHeader(f.tokenCode)
			return
		}
		_, _ = io.WriteString(w, `{"access_token":"tok`+string(rune('0'+n))+`","expires_in":3600,"token_type":"bearer"}`)
	})
	mux.HandleFunc("POST /v4/games", func(w http.ResponseWriter, r *http.Request) {
		n := f.apiCalls.Add(1)
		b, _ := io.ReadAll(r.Body)
		f.lastBody.Store(string(b))
		if r.Header.Get("Client-ID") != "id" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if f.apiStatus != nil {
			if code := f.apiStatus(n); code != http.StatusOK {
				w.WriteHeader(code)
				return
			}
		}
		_, _ = io.WriteString(w, f.response)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

const eldenJSON = `[{
  "id": 119133, "name": "Elden Ring",
  "cover": {"id": 1, "image_id": "co4jni"},
  "first_release_date": 1645747200,
  "genres": [{"id": 12, "name": "Role-playing (RPG)"}, {"id": 31, "name": "Adventure"}],
  "platforms": [{"abbreviation": "Series X|S"}, {"abbreviation": "PC"}, {"abbreviation": "PS4"}, {"abbreviation": "PS5"}, {"abbreviation": "XONE"}, {"abbreviation": "PC"}],
  "involved_companies": [
    {"company": {"name": "Bandai Namco"}, "developer": false},
    {"company": {"name": "FromSoftware"}, "developer": true}
  ],
  "total_rating": 94.6,
  "summary": "Rise, Tarnished."
}]`

func newTestClient(srv *httptest.Server, now func() time.Time) *Client {
	return New("id", "secret", WithEndpoints(srv.URL+"/token", srv.URL+"/v4"), WithClock(now))
}

func TestSearchMapsFields(t *testing.T) {
	f := &fakeIGDB{response: eldenJSON}
	c := newTestClient(f.server(t), time.Now)

	gs, err := c.Search(context.Background(), `elden "ring"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(gs) != 1 {
		t.Fatalf("wyników = %d", len(gs))
	}
	g := gs[0]
	if g.ID != 119133 || g.Title != "Elden Ring" || g.Developer != "FromSoftware" {
		t.Errorf("g = %+v", g)
	}
	if g.CoverURL != "https://images.igdb.com/igdb/image/upload/t_cover_big/co4jni.jpg" {
		t.Errorf("cover = %q", g.CoverURL)
	}
	if g.ReleaseDate == nil || g.ReleaseDate.Format("2006-01-02") != "2022-02-25" {
		t.Errorf("data = %v", g.ReleaseDate)
	}
	if g.Rating == nil || *g.Rating != 95 {
		t.Errorf("ocena = %v", g.Rating)
	}
	if got := strings.Join(g.Platforms, ","); got != "PC,PS5,PS4,Xbox Series,Xbox One" {
		t.Errorf("platformy = %s", got)
	}
	if got := strings.Join(g.Genres, ","); got != "Role-playing (RPG),Adventure" {
		t.Errorf("gatunki = %s", got)
	}

	body := f.lastBody.Load().(string)
	if !strings.HasPrefix(body, `search "elden \"ring\""; fields id,name,cover.image_id,`) || !strings.HasSuffix(body, "limit 12;") {
		t.Errorf("zapytanie = %s", body)
	}
}

func TestTokenIsCachedAndRefreshedBeforeExpiry(t *testing.T) {
	f := &fakeIGDB{response: `[]`}
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	c := newTestClient(f.server(t), func() time.Time { return now })
	ctx := context.Background()

	for range 3 {
		if _, err := c.Search(ctx, "x"); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.tokenCalls.Load(); n != 1 {
		t.Errorf("pobrań tokenu = %d, oczekiwano 1", n)
	}

	// 56 minut później token (ważny 60 min) jest w oknie odświeżania.
	now = now.Add(56 * time.Minute)
	if _, err := c.Search(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if n := f.tokenCalls.Load(); n != 2 {
		t.Errorf("pobrań tokenu = %d, oczekiwano 2", n)
	}
}

func TestRetriesOnceAfter401(t *testing.T) {
	f := &fakeIGDB{response: `[]`, apiStatus: func(call int32) int {
		if call == 1 {
			return http.StatusUnauthorized
		}
		return http.StatusOK
	}}
	c := newTestClient(f.server(t), time.Now)
	if _, err := c.Search(context.Background(), "x"); err != nil {
		t.Fatalf("err = %v", err)
	}
	if f.tokenCalls.Load() != 2 || f.apiCalls.Load() != 2 {
		t.Errorf("token = %d, api = %d", f.tokenCalls.Load(), f.apiCalls.Load())
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name string
		f    *fakeIGDB
		want error
	}{
		{"złe klucze", &fakeIGDB{tokenCode: http.StatusBadRequest}, ErrBadCredentials},
		{"twitch leży", &fakeIGDB{tokenCode: http.StatusServiceUnavailable}, ErrUnavailable},
		{"igdb 500", &fakeIGDB{apiStatus: func(int32) int { return 500 }}, ErrUnavailable},
		{"igdb limit", &fakeIGDB{apiStatus: func(int32) int { return http.StatusTooManyRequests }}, ErrUnavailable},
		{"ciągłe 401", &fakeIGDB{apiStatus: func(int32) int { return http.StatusUnauthorized }}, ErrBadCredentials},
		{"śmieci w odpowiedzi", &fakeIGDB{response: `<html>`}, ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(tt.f.server(t), time.Now)
			if _, err := c.Search(context.Background(), "x"); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, oczekiwano %v", err, tt.want)
			}
		})
	}
}

func TestUnreachable(t *testing.T) {
	c := New("id", "secret", WithEndpoints("http://127.0.0.1:1/token", "http://127.0.0.1:1/v4"))
	if _, err := c.Search(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
}

func TestGameByID(t *testing.T) {
	f := &fakeIGDB{response: eldenJSON}
	c := newTestClient(f.server(t), time.Now)
	g, err := c.Game(context.Background(), 119133)
	if err != nil || g.Summary != "Rise, Tarnished." {
		t.Fatalf("g = %+v, err = %v", g, err)
	}
	if body := f.lastBody.Load().(string); !strings.Contains(body, ",summary; where id = 119133; limit 1;") {
		t.Errorf("zapytanie = %s", body)
	}

	f.response = `[]`
	if _, err := c.Game(context.Background(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestPlatformLabels(t *testing.T) {
	got := SortPlatforms([]string{
		PlatformLabel("Switch", ""), PlatformLabel("", "Nintendo Switch 2"), PlatformLabel("Linux", ""),
		PlatformLabel("PC", ""), PlatformLabel("Dreamcast", ""), PlatformLabel("XONE", ""), "",
	})
	if s := strings.Join(got, ","); s != "PC,Xbox One,Switch 2,Switch,Linux,Dreamcast" {
		t.Errorf("platformy = %s", s)
	}
}
