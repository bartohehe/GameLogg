package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bartohehe/gamelogg/internal/igdb"
	"github.com/bartohehe/gamelogg/internal/spine"
	"github.com/bartohehe/gamelogg/internal/store"
)

// --- atrapy ---

type fakeGames struct {
	games      map[int64]igdb.Game
	err        error
	gameCalls  int
	lastSearch string
}

func (f *fakeGames) Search(_ context.Context, q string) ([]igdb.Game, error) {
	f.lastSearch = q
	if f.err != nil {
		return nil, f.err
	}
	var out []igdb.Game
	for _, g := range f.games {
		if strings.Contains(strings.ToLower(g.Title), strings.ToLower(q)) {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeGames) Game(_ context.Context, id int64) (igdb.Game, error) {
	f.gameCalls++
	if f.err != nil {
		return igdb.Game{}, f.err
	}
	g, ok := f.games[id]
	if !ok {
		return igdb.Game{}, igdb.ErrNotFound
	}
	return g, nil
}

type fakeSpines struct{ calls int }

func (f *fakeSpines) FromCover(_ context.Context, coverURL, title string) (spine.Colors, error) {
	f.calls++
	if coverURL == "" {
		return spine.Fallback(title), nil
	}
	return spine.Colors{Color: "#2E2A22", Accent: "#D7B55B", Ink: "#FFFFFF"}, nil
}

// fakeLibrary odwzorowuje zachowanie store.Store w pamięci.
type fakeLibrary struct {
	mu      sync.Mutex
	games   map[int64]store.Game // po igdb_id
	entries map[int64]store.LibraryEntry
	nextID  int64
}

func newFakeLibrary() *fakeLibrary {
	return &fakeLibrary{games: map[int64]store.Game{}, entries: map[int64]store.LibraryEntry{}, nextID: 1}
}

func (f *fakeLibrary) GameByIGDBID(_ context.Context, id int64) (store.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	g, ok := f.games[id]
	if !ok {
		return store.Game{}, store.ErrNotFound
	}
	return g, nil
}

func (f *fakeLibrary) UpsertGame(_ context.Context, g store.Game) (store.Game, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if old, ok := f.games[g.IGDBID]; ok {
		g.ID = old.ID
	} else {
		g.ID = int64(len(f.games) + 100)
	}
	g.FetchedAt = time.Now()
	f.games[g.IGDBID] = g
	return g, nil
}

func (f *fakeLibrary) gameByID(id int64) store.Game {
	for _, g := range f.games {
		if g.ID == id {
			return g
		}
	}
	return store.Game{}
}

func (f *fakeLibrary) Library(_ context.Context, uid int64, status store.Status, _ string) ([]store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []store.LibraryEntry{}
	for _, e := range f.entries {
		if e.UserID == uid && (status == "" || e.Status == status) {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeLibrary) Entry(_ context.Context, uid, id int64) (store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok || e.UserID != uid {
		return store.LibraryEntry{}, store.ErrNotFound
	}
	return e, nil
}

func (f *fakeLibrary) EntryByIGDBID(_ context.Context, uid, igdbID int64) (store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.entries {
		if e.UserID == uid && e.Game.IGDBID == igdbID {
			return e, nil
		}
	}
	return store.LibraryEntry{}, store.ErrNotFound
}

func (f *fakeLibrary) EntriesByIGDBIDs(_ context.Context, uid int64, ids []int64) (map[int64]store.EntryRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]store.EntryRef{}
	for _, e := range f.entries {
		if e.UserID == uid && slices.Contains(ids, e.Game.IGDBID) {
			out[e.Game.IGDBID] = store.EntryRef{ID: e.ID, Status: e.Status}
		}
	}
	return out, nil
}

func (f *fakeLibrary) AddToLibrary(_ context.Context, uid, gameID int64, st store.Status, platform string) (store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.entries {
		if e.UserID == uid && e.Game.ID == gameID {
			return store.LibraryEntry{}, store.ErrAlreadyInLibrary
		}
	}
	e := store.LibraryEntry{ID: f.nextID, UserID: uid, Status: st, Platform: platform, Game: f.gameByID(gameID),
		AddedAt: time.Now(), UpdatedAt: time.Now()}
	if st == store.StatusCompleted {
		e.CompletedCount = 1
	}
	f.entries[e.ID] = e
	f.nextID++
	return e, nil
}

func (f *fakeLibrary) UpdateEntry(_ context.Context, uid, id int64, p store.EntryPatch) (store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok || e.UserID != uid {
		return store.LibraryEntry{}, store.ErrNotFound
	}
	if p.Status != nil {
		e.Status = *p.Status
	}
	if p.Platform != nil {
		e.Platform = *p.Platform
	}
	if p.Note != nil {
		e.Note = *p.Note
	}
	f.entries[id] = e
	return e, nil
}

func (f *fakeLibrary) DeleteEntry(_ context.Context, uid, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.entries[id]
	if !ok || e.UserID != uid {
		return store.ErrNotFound
	}
	delete(f.entries, id)
	return nil
}

// --- środowisko testowe ---

type libEnv struct {
	h      http.Handler
	games  *fakeGames
	spines *fakeSpines
	lib    *fakeLibrary
}

func elden() igdb.Game {
	d := time.Date(2022, 2, 25, 0, 0, 0, 0, time.UTC)
	r := 95
	return igdb.Game{ID: 119133, Title: "Elden Ring", CoverURL: "https://images.igdb.com/x.jpg", ReleaseDate: &d,
		Developer: "FromSoftware", Genres: []string{"RPG"}, Platforms: []string{"PC", "PS5"}, Rating: &r, Summary: "Rise."}
}

func newLibEnv(t *testing.T, withIGDB bool) *libEnv {
	t.Helper()
	env := &libEnv{
		games:  &fakeGames{games: map[int64]igdb.Game{119133: elden(), 7: {ID: 7, Title: "Bez Platform"}}},
		spines: &fakeSpines{},
		lib:    newFakeLibrary(),
	}
	d := Deps{
		Log: testLogger(), DB: fakeDB{}, Users: newFakeUsers(), Library: env.lib, Spines: env.spines,
		Auth: AuthDeps{Enabled: false, OwnerID: 1},
	}
	if withIGDB {
		d.Games = env.games
	}
	env.h = NewRouter(d)
	return env
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatalf("dekodowanie: %v", err)
	}
	return v
}

// --- testy ---

func TestSearchGames(t *testing.T) {
	env := newLibEnv(t, true)

	rec := send(t, env.h, http.MethodGet, "/api/games/search?q=elden", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	body := decodeBody[struct{ Results []searchResultDTO }](t, rec)
	if len(body.Results) != 1 || body.Results[0].Title != "Elden Ring" || body.Results[0].Entry != nil {
		t.Fatalf("wyniki = %+v", body.Results)
	}
	if r := body.Results[0]; r.Year == nil || *r.Year != 2022 || r.Spine != nil {
		t.Errorf("wynik = %+v", r)
	}

	// Po dodaniu wynik ma „Na półce”.
	send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusWishlist, Platform: "PS5"})
	body = decodeBody[struct{ Results []searchResultDTO }](t, send(t, env.h, http.MethodGet, "/api/games/search?q=elden", nil))
	if e := body.Results[0].Entry; e == nil || e.Status != store.StatusWishlist {
		t.Errorf("entry = %+v", e)
	}

	// Za krótkie zapytanie nie idzie do IGDB.
	env.games.lastSearch = ""
	rec = send(t, env.h, http.MethodGet, "/api/games/search?q=e", nil)
	if rec.Code != http.StatusOK || env.games.lastSearch != "" {
		t.Errorf("krótkie zapytanie: %d, %q", rec.Code, env.games.lastSearch)
	}
}

func TestIGDBErrors(t *testing.T) {
	t.Run("brak kluczy", func(t *testing.T) {
		env := newLibEnv(t, false)
		for _, target := range []string{"/api/games/search?q=elden", "/api/games/119133"} {
			rec := send(t, env.h, http.MethodGet, target, nil)
			e := decodeErr(t, rec)
			if rec.Code != http.StatusServiceUnavailable || e.Code != "igdb_not_configured" || e.Message != msgIGDBNotConfigured {
				t.Errorf("%s: %d %+v", target, rec.Code, e)
			}
		}
		rec := send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"})
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("dodawanie bez kluczy: %d", rec.Code)
		}
	})

	tests := []struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		{igdb.ErrUnavailable, http.StatusBadGateway, "igdb_unavailable"},
		{igdb.ErrBadCredentials, http.StatusBadGateway, "igdb_bad_credentials"},
		{context.DeadlineExceeded, http.StatusBadGateway, "igdb_unavailable"},
	}
	for _, tt := range tests {
		env := newLibEnv(t, true)
		env.games.err = tt.err
		rec := send(t, env.h, http.MethodGet, "/api/games/search?q=elden", nil)
		if e := decodeErr(t, rec); rec.Code != tt.wantStatus || e.Code != tt.wantCode {
			t.Errorf("%v: %d %+v", tt.err, rec.Code, e)
		}
	}

	env := newLibEnv(t, true)
	env.games.err = igdb.ErrUnavailable
	if e := decodeErr(t, send(t, env.h, http.MethodGet, "/api/games/search?q=elden", nil)); e.Message != "IGDB nie odpowiada. Spróbuj za chwilę." {
		t.Errorf("komunikat = %q", e.Message)
	}
}

func TestAddToLibrary(t *testing.T) {
	env := newLibEnv(t, true)

	rec := send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusCompleted, Platform: "PS5"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	e := decodeBody[entryDTO](t, rec)
	if e.Status != store.StatusCompleted || e.Platform != "PS5" || e.Game.Title != "Elden Ring" || e.CompletedCount != 1 {
		t.Errorf("wpis = %+v", e)
	}
	if e.Game.Spine == nil || e.Game.Spine.Color != "#2E2A22" || e.Game.Spine.Ink != "#FFFFFF" {
		t.Errorf("grzbiet = %+v", e.Game.Spine)
	}
	if env.spines.calls != 1 {
		t.Errorf("kolorów liczonych %d razy", env.spines.calls)
	}

	// Duplikat.
	rec = send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"})
	if rec.Code != http.StatusConflict || decodeErr(t, rec).Code != "already_in_library" {
		t.Errorf("duplikat: %d", rec.Code)
	}
	// Druga próba korzysta z cache: bez ponownego IGDB i kolorów.
	if env.games.gameCalls != 1 || env.spines.calls != 1 {
		t.Errorf("IGDB %d razy, kolory %d razy", env.games.gameCalls, env.spines.calls)
	}
}

func TestAddValidation(t *testing.T) {
	env := newLibEnv(t, true)
	tests := []struct {
		name      string
		req       addRequest
		status    int
		code      string
		wantField string
	}{
		{"brak gry", addRequest{Status: store.StatusPlanned}, 400, "invalid_igdb_id", "igdbId"},
		{"zły status", addRequest{IGDBID: 119133, Status: "done", Platform: "PC"}, 400, "invalid_status", "status"},
		{"platforma spoza IGDB", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "Switch"}, 400, "invalid_platform", "platform"},
		{"pusta platforma", addRequest{IGDBID: 119133, Status: store.StatusPlanned}, 400, "invalid_platform", "platform"},
		{"nieznana gra", addRequest{IGDBID: 999, Status: store.StatusPlanned}, 404, "game_not_found", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := send(t, env.h, http.MethodPost, "/api/library", tt.req)
			if e := decodeErr(t, rec); rec.Code != tt.status || e.Code != tt.code || e.Field != tt.wantField {
				t.Errorf("%d %+v", rec.Code, e)
			}
		})
	}

	// Gra bez platform w IGDB: platforma musi zostać pusta.
	rec := send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 7, Status: store.StatusPlanned})
	if rec.Code != http.StatusCreated {
		t.Errorf("gra bez platform: %d %s", rec.Code, rec.Body)
	}
}

func TestPatchAndDeleteEntry(t *testing.T) {
	env := newLibEnv(t, true)
	e := decodeBody[entryDTO](t, send(t, env.h, http.MethodPost, "/api/library",
		addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"}))
	path := "/api/library/" + itoa(e.ID)

	note := "Drugie przejście."
	rec := send(t, env.h, http.MethodPatch, path, map[string]any{"status": "in_progress", "platform": "PS5", "note": note})
	got := decodeBody[entryDTO](t, rec)
	if rec.Code != http.StatusOK || got.Status != store.StatusInProgress || got.Platform != "PS5" || got.Note != note {
		t.Errorf("patch: %d %+v", rec.Code, got)
	}

	for _, tt := range []struct {
		body any
		code string
	}{
		{map[string]any{"status": "gotowe"}, "invalid_status"},
		{map[string]any{"platform": "Switch"}, "invalid_platform"},
		{map[string]any{"note": strings.Repeat("ą", maxNoteRunes+1)}, "note_too_long"},
	} {
		if e := decodeErr(t, send(t, env.h, http.MethodPatch, path, tt.body)); e.Code != tt.code {
			t.Errorf("oczekiwano %s, jest %+v", tt.code, e)
		}
	}

	if rec := send(t, env.h, http.MethodPatch, "/api/library/999", map[string]any{"note": "x"}); rec.Code != http.StatusNotFound {
		t.Errorf("nieistniejący wpis: %d", rec.Code)
	}

	lib := decodeBody[struct{ Items []entryDTO }](t, send(t, env.h, http.MethodGet, "/api/library?status=in_progress&sort=title", nil))
	if len(lib.Items) != 1 {
		t.Errorf("biblioteka = %+v", lib.Items)
	}
	if rec := send(t, env.h, http.MethodGet, "/api/library?sort=random", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("złe sortowanie: %d", rec.Code)
	}

	if rec := send(t, env.h, http.MethodDelete, path, nil); rec.Code != http.StatusNoContent {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := send(t, env.h, http.MethodDelete, path, nil); rec.Code != http.StatusNotFound {
		t.Errorf("ponowny delete: %d", rec.Code)
	}
}

func TestGameDetails(t *testing.T) {
	env := newLibEnv(t, true)

	// Gry nie ma w cache: dane prosto z IGDB, bez wpisu i bez grzbietu.
	d := decodeBody[gameDetailsDTO](t, send(t, env.h, http.MethodGet, "/api/games/119133", nil))
	if d.Title != "Elden Ring" || d.Summary != "Rise." || d.Entry != nil || d.Spine != nil || d.SummaryTranslated != nil {
		t.Errorf("szczegóły = %+v", d)
	}

	send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"})
	d = decodeBody[gameDetailsDTO](t, send(t, env.h, http.MethodGet, "/api/games/119133", nil))
	if d.Entry == nil || d.Entry.Status != store.StatusPlanned || d.Spine == nil {
		t.Errorf("po dodaniu = %+v", d)
	}

	if rec := send(t, env.h, http.MethodGet, "/api/games/abc", nil); rec.Code != http.StatusNotFound {
		t.Errorf("zły id: %d", rec.Code)
	}
	if rec := send(t, env.h, http.MethodGet, "/api/games/555", nil); rec.Code != http.StatusNotFound {
		t.Errorf("nieznana gra: %d", rec.Code)
	}
}

func TestEnsureGameFallsBackToStaleCache(t *testing.T) {
	env := newLibEnv(t, true)
	stale := store.Game{IGDBID: 119133, Title: "Elden Ring (stare)", Platforms: []string{"PC"},
		SpineColor: "#111111", SpineAccent: "#222222", SpineInk: "#FFFFFF"}
	g, _ := env.lib.UpsertGame(context.Background(), stale)
	g.FetchedAt = time.Now().Add(-60 * 24 * time.Hour)
	env.lib.games[119133] = g
	env.games.err = igdb.ErrUnavailable

	rec := send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if e := decodeBody[entryDTO](t, rec); e.Game.Title != "Elden Ring (stare)" {
		t.Errorf("tytuł = %q", e.Game.Title)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
