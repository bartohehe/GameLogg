package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func testGame(igdbID int64, title string) Game {
	d := time.Date(2022, 2, 25, 0, 0, 0, 0, time.UTC)
	r := 95
	return Game{
		IGDBID: igdbID, Title: title, CoverURL: "https://example.com/c.jpg", ReleaseDate: &d,
		Developer: "Studio", Genres: []string{"RPG"}, Platforms: []string{"PC", "PS5"}, IGDBRating: &r,
		Summary: "Opis.", SpineColor: "#2E2A22", SpineAccent: "#D7B55B", SpineInk: "#FFFFFF",
	}
}

func seedUser(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	u, err := s.CreateUser(context.Background(), name, name+"@example.com", "h")
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func seedGame(t *testing.T, s *Store, igdbID int64, title string) Game {
	t.Helper()
	g, err := s.UpsertGame(context.Background(), testGame(igdbID, title))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func migratedStoreWithGames(t *testing.T) *Store {
	t.Helper()
	s := migratedStore(t)
	if _, err := s.pool.Exec(context.Background(), `TRUNCATE games RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUpsertGame(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()

	g := seedGame(t, s, 119133, "Elden Ring")
	if g.ID == 0 || g.Title != "Elden Ring" || len(g.Platforms) != 2 || *g.IGDBRating != 95 {
		t.Errorf("g = %+v", g)
	}

	upd := testGame(119133, "ELDEN RING")
	upd.CoverURL = ""
	g2, err := s.UpsertGame(ctx, upd)
	if err != nil {
		t.Fatal(err)
	}
	if g2.ID != g.ID || g2.Title != "ELDEN RING" || g2.CoverURL != "" {
		t.Errorf("po aktualizacji: %+v", g2)
	}

	got, err := s.GameByIGDBID(ctx, 119133)
	if err != nil || got.ID != g.ID {
		t.Errorf("GameByIGDBID = %+v, %v", got, err)
	}
	if _, err := s.GameByIGDBID(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestLibraryAddListSort(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	other := seedUser(t, s, "ewa")

	b := seedGame(t, s, 2, "baldur's Gate 3")
	a := seedGame(t, s, 1, "Alan Wake 2")
	c := seedGame(t, s, 3, "Celeste")

	eb, err := s.AddToLibrary(ctx, u, b.ID, StatusCompleted, "PC")
	if err != nil {
		t.Fatal(err)
	}
	if eb.CompletedCount != 1 || eb.FinishedAt == nil || eb.Game.Title != "baldur's Gate 3" {
		t.Errorf("ukończona od razu: %+v", eb)
	}
	ea, _ := s.AddToLibrary(ctx, u, a.ID, StatusPlanned, "PS5")
	if ea.CompletedCount != 0 || ea.FinishedAt != nil {
		t.Errorf("planowana: %+v", ea)
	}
	if _, err := s.AddToLibrary(ctx, u, c.ID, StatusAbandoned, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddToLibrary(ctx, other, a.ID, StatusWishlist, ""); err != nil {
		t.Fatal(err)
	}

	if _, err := s.AddToLibrary(ctx, u, a.ID, StatusWishlist, ""); !errors.Is(err, ErrAlreadyInLibrary) {
		t.Errorf("duplikat: err = %v", err)
	}

	// Ocena 90 dla ukończonej, żeby sprawdzić sortowanie po ocenie.
	if _, err := s.pool.Exec(ctx, `UPDATE user_games SET score = 90 WHERE id = $1`, eb.ID); err != nil {
		t.Fatal(err)
	}

	titles := func(es []LibraryEntry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.Game.Title
		}
		return out
	}
	tests := []struct {
		status Status
		sort   string
		want   []string
	}{
		{"", "score", []string{"baldur's Gate 3", "Alan Wake 2", "Celeste"}},
		{"", "title", []string{"Alan Wake 2", "baldur's Gate 3", "Celeste"}},
		{"", "added", []string{"Celeste", "Alan Wake 2", "baldur's Gate 3"}},
		{StatusPlanned, "score", []string{"Alan Wake 2"}},
		{StatusOnHold, "score", []string{}},
	}
	for _, tt := range tests {
		got, err := s.Library(ctx, u, tt.status, tt.sort)
		if err != nil {
			t.Fatal(err)
		}
		if g := titles(got); len(g) != len(tt.want) || (len(g) > 0 && g[0] != tt.want[0]) || (len(g) > 2 && g[2] != tt.want[2]) {
			t.Errorf("status=%q sort=%s: %v, oczekiwano %v", tt.status, tt.sort, g, tt.want)
		}
	}

	refs, err := s.EntriesByIGDBIDs(ctx, u, []int64{1, 2, 999})
	if err != nil || len(refs) != 2 || refs[1].Status != StatusPlanned || refs[2].ID != eb.ID {
		t.Errorf("refs = %+v, %v", refs, err)
	}
	if e, err := s.EntryByIGDBID(ctx, u, 1); err != nil || e.ID != ea.ID {
		t.Errorf("EntryByIGDBID = %+v, %v", e, err)
	}
}

func TestUpdateEntryStatusTransitions(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	g1 := seedGame(t, s, 1, "Jedna")
	g2 := seedGame(t, s, 2, "Druga")
	g3 := seedGame(t, s, 3, "Trzecia")

	e1, _ := s.AddToLibrary(ctx, u, g1.ID, StatusInProgress, "PC")
	e2, _ := s.AddToLibrary(ctx, u, g2.ID, StatusCompleted, "PC")
	e3, _ := s.AddToLibrary(ctx, u, g3.ID, StatusCompleted, "PC")

	st := func(s Status) *Status { return &s }

	// W trakcie → ukończona: licznik 1, data ukończenia.
	e1, err := s.UpdateEntry(ctx, u, e1.ID, EntryPatch{Status: st(StatusCompleted)})
	if err != nil || e1.CompletedCount != 1 || e1.FinishedAt == nil {
		t.Fatalf("ukończenie: %+v, %v", e1, err)
	}

	// Ranking gameplay: e1 (0), e2 (1), e3 (2); ocena e1 = 100.
	_, err = s.pool.Exec(ctx, `
		INSERT INTO category_rankings (user_id, category, user_game_id, position) VALUES
		($1, 'gameplay', $2, 0), ($1, 'gameplay', $3, 1), ($1, 'gameplay', $4, 2), ($1, 'story', $2, 0)`,
		u, e1.ID, e2.ID, e3.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, `UPDATE user_games SET score = 100 WHERE id = $1`, e1.ID); err != nil {
		t.Fatal(err)
	}

	// Ukończona → w trakcie: ocena znika, gra wypada z rankingów, pozycje się zagęszczają.
	e1, err = s.UpdateEntry(ctx, u, e1.ID, EntryPatch{Status: st(StatusInProgress)})
	if err != nil || e1.Score != nil || e1.FinishedAt != nil || e1.CompletedCount != 1 {
		t.Fatalf("cofnięcie: %+v, %v", e1, err)
	}
	positions := map[int64]int{}
	rows, _ := s.pool.Query(ctx, `SELECT user_game_id, position FROM category_rankings WHERE user_id = $1 ORDER BY category, position`, u)
	for rows.Next() {
		var id int64
		var p int
		_ = rows.Scan(&id, &p)
		positions[id] = p
	}
	rows.Close()
	if len(positions) != 2 || positions[e2.ID] != 0 || positions[e3.ID] != 1 {
		t.Errorf("pozycje po usunięciu = %v", positions)
	}

	// Drugie ukończenie: licznik 2. Porzucenie: finished_at ustawione, licznik bez zmian.
	e1, _ = s.UpdateEntry(ctx, u, e1.ID, EntryPatch{Status: st(StatusCompleted)})
	if e1.CompletedCount != 2 {
		t.Errorf("drugie ukończenie: licznik = %d", e1.CompletedCount)
	}
	e1, _ = s.UpdateEntry(ctx, u, e1.ID, EntryPatch{Status: st(StatusAbandoned)})
	if e1.CompletedCount != 2 || e1.FinishedAt == nil || e1.Status != StatusAbandoned {
		t.Errorf("porzucenie: %+v", e1)
	}

	// Ten sam status: nic się nie zmienia (licznik nie rośnie).
	e2b, _ := s.UpdateEntry(ctx, u, e2.ID, EntryPatch{Status: st(StatusCompleted)})
	if e2b.CompletedCount != 1 {
		t.Errorf("ten sam status podbił licznik: %d", e2b.CompletedCount)
	}

	// Platforma i notatka.
	pl, note := "PS5", "Drugie przejście."
	e2b, err = s.UpdateEntry(ctx, u, e2.ID, EntryPatch{Platform: &pl, Note: &note})
	if err != nil || e2b.Platform != "PS5" || e2b.Note != note || e2b.Status != StatusCompleted {
		t.Errorf("platforma i notatka: %+v, %v", e2b, err)
	}
}

func TestDeleteEntryCompactsRankings(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	var ids []int64
	for i, title := range []string{"A", "B", "C"} {
		g := seedGame(t, s, int64(i+1), title)
		e, _ := s.AddToLibrary(ctx, u, g.ID, StatusCompleted, "")
		ids = append(ids, e.ID)
		if _, err := s.pool.Exec(ctx, `INSERT INTO category_rankings (user_id, category, user_game_id, position) VALUES ($1, 'audio', $2, $3)`, u, e.ID, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteEntry(ctx, u, ids[0]); err != nil {
		t.Fatal(err)
	}
	var minPos, n int
	_ = s.pool.QueryRow(ctx, `SELECT min(position), count(*) FROM category_rankings WHERE user_id = $1`, u).Scan(&minPos, &n)
	if minPos != 0 || n != 2 {
		t.Errorf("po usunięciu: min = %d, n = %d", minPos, n)
	}
	if err := s.DeleteEntry(ctx, u, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("ponowne usunięcie: err = %v", err)
	}
}

func TestLibraryIsolatedPerUser(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	owner := seedUser(t, s, "jan")
	intruder := seedUser(t, s, "ewa")
	g := seedGame(t, s, 1, "Gra")
	e, _ := s.AddToLibrary(ctx, owner, g.ID, StatusPlanned, "")

	note := "cudza"
	if _, err := s.UpdateEntry(ctx, intruder, e.ID, EntryPatch{Note: &note}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update cudzego: err = %v", err)
	}
	if err := s.DeleteEntry(ctx, intruder, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete cudzego: err = %v", err)
	}
	if _, err := s.Entry(ctx, intruder, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("odczyt cudzego: err = %v", err)
	}
	if lib, _ := s.Library(ctx, intruder, "", ""); len(lib) != 0 {
		t.Errorf("cudza biblioteka widoczna: %d", len(lib))
	}
}
