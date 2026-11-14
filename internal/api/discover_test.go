package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"testing"

	"github.com/bartohehe/gamelogg/internal/igdb"
	"github.com/bartohehe/gamelogg/internal/store"
)

// --- atrapy ---

func (f *fakeGames) Popular(context.Context) ([]igdb.Game, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []igdb.Game{f.games[119133], f.games[7]}, nil
}

func (f *fakeGames) Similar(_ context.Context, ids []int64) (map[int64][]igdb.Game, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := map[int64][]igdb.Game{}
	for _, id := range ids {
		out[id] = f.similar[id]
	}
	return out, nil
}

func (f *fakeLibrary) TopRated(_ context.Context, uid int64, limit int) ([]store.LibraryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.LibraryEntry
	for _, e := range f.entries {
		if e.UserID == uid && e.Score != nil {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return *out[i].Score > *out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeLibrary) Stats(_ context.Context, uid int64, year int) (store.Stats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := store.Stats{Counts: map[store.Status]int{}, Year: store.YearStats{Year: year}}
	for _, e := range f.entries {
		if e.UserID == uid {
			st.Counts[e.Status]++
			st.Total++
		}
	}
	return st, nil
}

type fakeTranslator struct {
	err   error
	calls int
}

func (f *fakeTranslator) ToPolish(_ context.Context, text string) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return "PL: " + text, nil
}

// --- testy ---

func TestPopularMarksOwned(t *testing.T) {
	env := newLibEnv(t, true)
	send(t, env.h, http.MethodPost, "/api/library", addRequest{IGDBID: 119133, Status: store.StatusPlanned, Platform: "PC"})
	body := decodeBody[struct{ Results []searchResultDTO }](t, send(t, env.h, http.MethodGet, "/api/games/popular", nil))
	if len(body.Results) != 2 || body.Results[0].Entry == nil || body.Results[0].Entry.Status != store.StatusPlanned || body.Results[1].Entry != nil {
		t.Errorf("popularne = %+v", body.Results)
	}
	env.games.err = igdb.ErrUnavailable
	if rec := send(t, env.h, http.MethodGet, "/api/games/popular", nil); rec.Code != http.StatusBadGateway {
		t.Errorf("IGDB leży: %d", rec.Code)
	}
}

func TestRecommendations(t *testing.T) {
	env := newLibEnv(t, true)
	g := func(id int64, title string) igdb.Game {
		return igdb.Game{ID: id, Title: title, Platforms: []string{"PC"}}
	}
	for _, x := range []igdb.Game{g(2, "Druga"), g(3, "Trzecia"), g(50, "Lies of P"), g(51, "Sekiro"), g(52, "Nioh"), g(60, "Pentiment")} {
		env.games.games[x.ID] = x
	}
	// Elden Ring (ocena 100) → Lies of P, Sekiro, Druga (już na półce); Druga (60) → Sekiro (duplikat), Pentiment.
	env.games.similar = map[int64][]igdb.Game{
		119133: {env.games.games[50], env.games.games[51], env.games.games[2], env.games.games[52]},
		2:      {env.games.games[51], env.games.games[60]},
	}

	empty := decodeBody[struct {
		Items   []recommendationDTO
		BasedOn int
	}](t, send(t, env.h, http.MethodGet, "/api/games/recommendations", nil))
	if len(empty.Items) != 0 || empty.BasedOn != 0 {
		t.Errorf("bez ocen: %+v", empty)
	}

	a, b := addCompleted(t, env, 119133), addCompleted(t, env, 2)
	send(t, env.h, http.MethodPut, "/api/rankings/gameplay", map[string]any{"userGameIds": []int64{a, b}})

	got := decodeBody[struct {
		Items   []recommendationDTO
		BasedOn int
	}](t, send(t, env.h, http.MethodGet, "/api/games/recommendations", nil))
	titles := []string{}
	for _, it := range got.Items {
		titles = append(titles, it.Game.Title+"←"+it.Because.Title+"/"+itoa(int64(it.Because.Score)))
	}
	// Na zmianę z każdej gry źródłowej, bez gier z półki i bez powtórek.
	want := []string{"Lies of P←Elden Ring/100", "Sekiro←Druga/60", "Nioh←Elden Ring/100", "Pentiment←Druga/60"}
	if got.BasedOn != 2 || len(titles) != len(want) {
		t.Fatalf("polecenia = %v (źródeł %d)", titles, got.BasedOn)
	}
	for i := range want {
		if titles[i] != want[i] {
			t.Errorf("polecenie %d = %s, oczekiwano %s", i, titles[i], want[i])
		}
	}
}

func TestStatsEndpoint(t *testing.T) {
	env := newLibEnv(t, true)
	addCompleted(t, env, 119133)
	st := decodeBody[statsDTO](t, send(t, env.h, http.MethodGet, "/api/stats?year=2026", nil))
	if st.Total != 1 || st.Counts[store.StatusCompleted] != 1 || st.Year.Year != 2026 {
		t.Errorf("stats = %+v", st)
	}
	if rec := send(t, env.h, http.MethodGet, "/api/stats?year=abc", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("zły rok: %d", rec.Code)
	}
}

func TestGameDetailsTranslation(t *testing.T) {
	newEnv := func(tr Translator) *libEnv {
		env := newLibEnv(t, true)
		env.h = NewRouter(Deps{Log: testLogger(), DB: fakeDB{}, Users: newFakeUsers(), Library: env.lib,
			Games: env.games, Spines: env.spines, Translator: tr, Auth: AuthDeps{OwnerID: 1}})
		return env
	}

	ok := &fakeTranslator{}
	d := decodeBody[gameDetailsDTO](t, send(t, newEnv(ok).h, http.MethodGet, "/api/games/119133", nil))
	if d.SummaryTranslated == nil || *d.SummaryTranslated != "PL: Rise." || d.Summary != "Rise." {
		t.Errorf("z tłumaczeniem: %+v", d)
	}

	failing := &fakeTranslator{err: errors.New("quota")}
	d = decodeBody[gameDetailsDTO](t, send(t, newEnv(failing).h, http.MethodGet, "/api/games/119133", nil))
	if d.SummaryTranslated != nil || d.Summary != "Rise." {
		t.Errorf("błąd tłumaczenia: %+v", d)
	}

	d = decodeBody[gameDetailsDTO](t, send(t, newEnv(nil).h, http.MethodGet, "/api/games/119133", nil))
	if d.SummaryTranslated != nil {
		t.Errorf("tłumaczenie wyłączone: %+v", d)
	}
}
