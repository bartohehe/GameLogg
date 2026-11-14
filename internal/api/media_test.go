package api

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bartohehe/GameLogg/internal/jikan"
	"github.com/bartohehe/GameLogg/internal/store"
	"github.com/bartohehe/GameLogg/internal/tmdb"
)

type fakeMovies struct{ err error }

func ip(v int) *int { return &v }

var dune = tmdb.Title{ID: 693134, Kind: tmdb.Movie, Title: "Diuna: Część druga", Year: ip(2024), PosterURL: "https://img/d.jpg", Score: ip(82)}
var prophecy = tmdb.Title{ID: 90228, Kind: tmdb.TV, Title: "Diuna: Proroctwo", Year: ip(2024)}

func (f *fakeMovies) Search(context.Context, string) ([]tmdb.Title, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []tmdb.Title{dune, prophecy}, nil
}

func (f *fakeMovies) Details(_ context.Context, kind tmdb.Kind, id int64) (tmdb.Title, error) {
	switch {
	case kind == tmdb.Movie && id == dune.ID:
		d := dune
		d.Creator, d.RuntimeMin = "Denis Villeneuve", ip(166)
		return d, nil
	case kind == tmdb.TV && id == prophecy.ID:
		p := prophecy
		p.Creator, p.Episodes = "Diane Ademu-John", ip(6)
		return p, nil
	}
	return tmdb.Title{}, tmdb.ErrNotFound
}

type fakeAnime struct{ err error }

var frieren = jikan.Anime{ID: 52991, Title: "Frieren", Year: ip(2023), Studios: "Madhouse", Episodes: ip(3), Score: ip(93)}

func (f *fakeAnime) Search(context.Context, string) ([]jikan.Anime, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []jikan.Anime{frieren}, nil
}

func (f *fakeAnime) Anime(_ context.Context, id int64) (jikan.Anime, error) {
	if id == frieren.ID {
		return frieren, nil
	}
	return jikan.Anime{}, jikan.ErrNotFound
}

// fakeMedia odwzorowuje store w pamięci (bez transakcji).
type fakeMedia struct {
	mu    sync.Mutex
	items map[int64]store.Media
	next  int64
}

func (f *fakeMedia) AddMedia(_ context.Context, m store.Media) (store.Media, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, x := range f.items {
		if x.UserID == m.UserID && x.Source == m.Source && x.ExternalID == m.ExternalID {
			return store.Media{}, store.ErrAlreadyInLibrary
		}
	}
	f.next++
	m.ID, m.AddedAt, m.UpdatedAt = f.next, time.Now(), time.Now()
	if m.Type != store.TypeFilm && m.Status == store.MediaWatched {
		m.WatchedEpisodes = m.Episodes
	}
	f.items[m.ID] = m
	return m, nil
}

func (f *fakeMedia) MediaList(_ context.Context, uid int64, typ store.MediaType, st store.MediaStatus) ([]store.Media, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.Media{}
	for _, m := range f.items {
		if m.UserID == uid && (typ == "" || m.Type == typ) && (st == "" || m.Status == st) {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeMedia) MediaItem(_ context.Context, uid, id int64) (store.Media, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok || m.UserID != uid {
		return store.Media{}, store.ErrNotFound
	}
	return m, nil
}

func (f *fakeMedia) MediaRefs(_ context.Context, uid int64, src store.MediaSource, ids []string) (map[string]store.MediaRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]store.MediaRef{}
	for _, m := range f.items {
		for _, id := range ids {
			if m.UserID == uid && m.Source == src && m.ExternalID == id {
				out[id] = store.MediaRef{ID: m.ID, Status: m.Status}
			}
		}
	}
	return out, nil
}

func (f *fakeMedia) UpdateMedia(_ context.Context, uid, id int64, p store.MediaPatch) (store.Media, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok || m.UserID != uid {
		return store.Media{}, store.ErrNotFound
	}
	if p.WatchedEpisodes != nil && m.Type == store.TypeFilm {
		return store.Media{}, store.ErrNotEpisodic
	}
	if p.Status != nil {
		m.Status = *p.Status
	}
	if p.WatchedEpisodes != nil {
		m.WatchedEpisodes = p.WatchedEpisodes
	}
	if p.ScoreSet {
		m.Score = p.Score
	}
	if p.Note != nil {
		m.Note = *p.Note
	}
	f.items[id] = m
	return m, nil
}

func (f *fakeMedia) IncrementEpisode(_ context.Context, uid, id int64) (store.Media, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok || m.UserID != uid {
		return store.Media{}, store.ErrNotFound
	}
	if m.Type == store.TypeFilm {
		return store.Media{}, store.ErrNotEpisodic
	}
	w := 1
	if m.WatchedEpisodes != nil {
		w = *m.WatchedEpisodes + 1
	}
	if m.Episodes != nil && w > *m.Episodes {
		return store.Media{}, store.ErrAllWatched
	}
	m.WatchedEpisodes, m.Status = &w, store.MediaWatching
	if m.Episodes != nil && w == *m.Episodes {
		m.Status = store.MediaWatched
	}
	f.items[id] = m
	return m, nil
}

func (f *fakeMedia) DeleteMedia(_ context.Context, uid, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.items[id]
	if !ok || m.UserID != uid {
		return store.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

type mediaEnv struct {
	h      http.Handler
	movies *fakeMovies
	anime  *fakeAnime
}

func newMediaEnv(t *testing.T, enabled, withTMDB bool) *mediaEnv {
	t.Helper()
	env := &mediaEnv{movies: &fakeMovies{}, anime: &fakeAnime{}}
	d := Deps{
		Log: testLogger(), DB: fakeDB{}, Users: newFakeUsers(), Library: newFakeLibrary(), Spines: &fakeSpines{},
		Media: &fakeMedia{items: map[int64]store.Media{}}, Anime: env.anime,
		Auth: AuthDeps{OwnerID: 1}, Features: Features{MediaEnabled: enabled, TMDBConfigured: withTMDB},
	}
	if withTMDB {
		d.Movies = env.movies
	}
	env.h = NewRouter(d)
	return env
}

type mediaSearchBody struct {
	Results        []mediaResultDTO
	TMDBConfigured bool `json:"tmdbConfigured"`
	Errors         []sourceErrorDTO
}

func TestMediaSearch(t *testing.T) {
	env := newMediaEnv(t, true, true)
	body := decodeBody[mediaSearchBody](t, send(t, env.h, http.MethodGet, "/api/media/search?q=diuna", nil))
	if len(body.Results) != 3 || !body.TMDBConfigured || len(body.Errors) != 0 {
		t.Fatalf("wyniki = %+v", body)
	}
	if r := body.Results[0]; r.Type != store.TypeFilm || r.Source != store.SourceTMDB || r.ExternalID != "693134" {
		t.Errorf("film = %+v", r)
	}
	if r := body.Results[2]; r.Type != store.TypeAnime || r.Source != store.SourceJikan || r.Creator != "Madhouse" {
		t.Errorf("anime = %+v", r)
	}

	only := decodeBody[mediaSearchBody](t, send(t, env.h, http.MethodGet, "/api/media/search?q=diuna&type=serial", nil))
	if len(only.Results) != 1 || only.Results[0].Title != "Diuna: Proroctwo" {
		t.Errorf("tylko seriale = %+v", only.Results)
	}

	// Jedno źródło leży: wyniki z drugiego i błąd w polu errors.
	env.anime.err = jikan.ErrUnavailable
	partial := decodeBody[mediaSearchBody](t, send(t, env.h, http.MethodGet, "/api/media/search?q=diuna", nil))
	if len(partial.Results) != 2 || len(partial.Errors) != 1 || partial.Errors[0].Source != store.SourceJikan {
		t.Errorf("częściowe = %+v", partial)
	}
	// Oba leżą: 502.
	env.movies.err = tmdb.ErrBadKey
	rec := send(t, env.h, http.MethodGet, "/api/media/search?q=diuna", nil)
	if rec.Code != http.StatusBadGateway || decodeErr(t, rec).Code != "tmdb_bad_key" {
		t.Errorf("oba źródła: %d", rec.Code)
	}
}

func TestMediaSearchWithoutTMDB(t *testing.T) {
	env := newMediaEnv(t, true, false)
	body := decodeBody[mediaSearchBody](t, send(t, env.h, http.MethodGet, "/api/media/search?q=frieren", nil))
	if body.TMDBConfigured || len(body.Results) != 1 || body.Results[0].Type != store.TypeAnime {
		t.Errorf("bez TMDB: %+v", body)
	}
	rec := send(t, env.h, http.MethodGet, "/api/media/search?q=diuna&type=film", nil)
	if e := decodeErr(t, rec); rec.Code != http.StatusServiceUnavailable || e.Code != "tmdb_not_configured" {
		t.Errorf("filmy bez klucza: %d %+v", rec.Code, e)
	}
}

func TestMediaDisabledIs404(t *testing.T) {
	env := newMediaEnv(t, false, true)
	for _, path := range []string{"/api/media", "/api/media/search?q=diuna"} {
		if rec := send(t, env.h, http.MethodGet, path, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

func TestMediaLifecycle(t *testing.T) {
	env := newMediaEnv(t, true, true)
	add := func(src, ext, typ, status string) (int, mediaDTO) {
		rec := send(t, env.h, http.MethodPost, "/api/media", map[string]string{"source": src, "externalId": ext, "type": typ, "status": status})
		if rec.Code != http.StatusCreated {
			return rec.Code, mediaDTO{}
		}
		return rec.Code, decodeBody[mediaDTO](t, rec)
	}

	code, film := add("tmdb", "693134", "film", "")
	if code != 201 || film.Status != store.MediaBacklog || film.Creator != "Denis Villeneuve" || *film.RuntimeMin != 166 || film.Spine.Color == "" {
		t.Fatalf("film: %d %+v", code, film)
	}
	_, serial := add("tmdb", "90228", "serial", "watching")
	if serial.Type != store.TypeSerial || *serial.Episodes != 6 {
		t.Errorf("serial = %+v", serial)
	}
	_, anime := add("jikan", "52991", "anime", "backlog")

	for _, tt := range []struct {
		src, ext, typ, status string
		want                  int
	}{
		{"tmdb", "693134", "film", "", 409},
		{"tmdb", "1", "film", "", 404},
		{"tmdb", "693134", "anime", "", 400},
		{"imdb", "1", "film", "", 400},
		{"jikan", "abc", "anime", "", 400},
		{"jikan", "52991", "anime", "done", 400},
	} {
		if code, _ := add(tt.src, tt.ext, tt.typ, tt.status); code != tt.want {
			t.Errorf("%+v: %d", tt, code)
		}
	}

	path := "/api/media/" + strconv.FormatInt(anime.ID, 10)
	for i := 1; i <= 3; i++ {
		rec := send(t, env.h, http.MethodPost, path+"/episode", nil)
		got := decodeBody[mediaDTO](t, rec)
		if *got.WatchedEpisodes != i {
			t.Errorf("odcinek %d: %+v", i, got)
		}
		if i == 3 && got.Status != store.MediaWatched {
			t.Errorf("po ostatnim odcinku status = %s", got.Status)
		}
	}
	if rec := send(t, env.h, http.MethodPost, path+"/episode", nil); rec.Code != http.StatusConflict {
		t.Errorf("ponad odcinki: %d", rec.Code)
	}
	if rec := send(t, env.h, http.MethodPost, "/api/media/"+strconv.FormatInt(film.ID, 10)+"/episode", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("film +1: %d", rec.Code)
	}

	got := decodeBody[mediaDTO](t, send(t, env.h, http.MethodPatch, path, map[string]any{"score": 93, "note": "Piękne."}))
	if *got.Score != 93 || got.Note != "Piękne." {
		t.Errorf("ocena: %+v", got)
	}
	got = decodeBody[mediaDTO](t, send(t, env.h, http.MethodPatch, path, map[string]any{"score": nil}))
	if got.Score != nil {
		t.Errorf("ocena nie wyczyszczona: %v", *got.Score)
	}
	for _, body := range []map[string]any{{"score": 101}, {"score": 7.5}, {"score": "dużo"}, {"watchedEpisodes": 4}, {"watchedEpisodes": -1}, {"status": "seen"}} {
		if rec := send(t, env.h, http.MethodPatch, path, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: %d", body, rec.Code)
		}
	}

	list := decodeBody[struct{ Items []mediaDTO }](t, send(t, env.h, http.MethodGet, "/api/media?type=serial", nil))
	if len(list.Items) != 1 || list.Items[0].ID != serial.ID {
		t.Errorf("filtr = %+v", list.Items)
	}
	search := decodeBody[mediaSearchBody](t, send(t, env.h, http.MethodGet, "/api/media/search?q=diuna", nil))
	if e := search.Results[0].Entry; e == nil || e.Status != store.MediaBacklog {
		t.Errorf("„Na półce” w wynikach = %+v", e)
	}

	if rec := send(t, env.h, http.MethodDelete, path, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := send(t, env.h, http.MethodDelete, path, nil); rec.Code != http.StatusNotFound {
		t.Errorf("ponowny delete: %d", rec.Code)
	}
}
