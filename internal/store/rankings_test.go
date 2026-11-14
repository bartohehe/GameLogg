package store

import (
	"context"
	"errors"
	"testing"

	"github.com/bartohehe/gamelogg/internal/ranking"
)

func scoreOf(t *testing.T, s *Store, uid, id int64) *int {
	t.Helper()
	e, err := s.Entry(context.Background(), uid, id)
	if err != nil {
		t.Fatal(err)
	}
	return e.Score
}

func eq(a *int, b int) bool { return a != nil && *a == b }

// seedCompleted dodaje n ukończonych gier i zwraca identyfikatory wpisów.
func seedCompleted(t *testing.T, s *Store, uid int64, titles ...string) []int64 {
	t.Helper()
	ids := make([]int64, len(titles))
	for i, title := range titles {
		g := seedGame(t, s, int64(1000+i), title)
		e, err := s.AddToLibrary(context.Background(), uid, g.ID, StatusCompleted, "")
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = e.ID
	}
	return ids
}

func TestSetRankingRecomputesAllScores(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	ids := seedCompleted(t, s, u, "A", "B", "C")
	a, b, c := ids[0], ids[1], ids[2]

	scores, err := s.SetRanking(ctx, u, ranking.Gameplay, []int64{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !eq(scores[a], 100) || !eq(scores[b], 60) || scores[c] != nil {
		t.Errorf("po pierwszym rankingu: a=%v b=%v c=%v", scores[a], scores[b], scores[c])
	}
	if !eq(scoreOf(t, s, u, a), 100) || scoreOf(t, s, u, c) != nil {
		t.Error("oceny nie zapisały się w user_games")
	}

	// Wstawienie C w środek przesuwa B: n = 3.
	if _, err := s.SetRanking(ctx, u, ranking.Gameplay, []int64{a, c, b}); err != nil {
		t.Fatal(err)
	}
	if !eq(scoreOf(t, s, u, b), 60) || !eq(scoreOf(t, s, u, c), 80) {
		t.Errorf("po wstawieniu: b=%v c=%v", scoreOf(t, s, u, b), scoreOf(t, s, u, c))
	}

	// Druga kategoria: średnia ważona tylko z kategorii z miejscem.
	if _, err := s.SetRanking(ctx, u, ranking.Story, []int64{b, a}); err != nil {
		t.Fatal(err)
	}
	// a: gameplay 100 (35), story 60 (25) → 83; b: 60·35 + 100·25 → 76,67 → 77; c: tylko gameplay 80.
	if !eq(scoreOf(t, s, u, a), 83) || !eq(scoreOf(t, s, u, b), 77) || !eq(scoreOf(t, s, u, c), 80) {
		t.Errorf("dwie kategorie: a=%v b=%v c=%v", scoreOf(t, s, u, a), scoreOf(t, s, u, b), scoreOf(t, s, u, c))
	}

	got, err := s.Ranking(ctx, u, ranking.Gameplay)
	if err != nil || len(got) != 3 || got[0].Entry.ID != a || got[1].Entry.ID != c || got[2].Position != 3 {
		t.Errorf("Ranking = %+v, %v", got, err)
	}

	places, sizes, err := s.EntryPlaces(ctx, u, c)
	if err != nil {
		t.Fatal(err)
	}
	if places[ranking.Gameplay] != (ranking.Place{Pos: 2, N: 3}) || len(places) != 1 || sizes[ranking.Story] != 2 || sizes[ranking.Audio] != 0 {
		t.Errorf("EntryPlaces = %+v, %+v", places, sizes)
	}

	// Pusta lista czyści kategorię.
	if _, err := s.SetRanking(ctx, u, ranking.Story, nil); err != nil {
		t.Fatal(err)
	}
	if !eq(scoreOf(t, s, u, a), 100) {
		t.Errorf("po wyczyszczeniu story: a=%v", scoreOf(t, s, u, a))
	}
}

func TestStatusChangeRemovesFromRankingsAndRecomputes(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	ids := seedCompleted(t, s, u, "A", "B", "C")
	a, b, c := ids[0], ids[1], ids[2]
	if _, err := s.SetRanking(ctx, u, ranking.Gameplay, []int64{a, b, c}); err != nil {
		t.Fatal(err)
	}
	if !eq(scoreOf(t, s, u, b), 80) {
		t.Fatalf("b = %v", scoreOf(t, s, u, b))
	}

	st := StatusInProgress
	if _, err := s.UpdateEntry(ctx, u, a, EntryPatch{Status: &st}); err != nil {
		t.Fatal(err)
	}
	// Bez A: B jest pierwsza z dwóch, C ostatnia.
	if scoreOf(t, s, u, a) != nil || !eq(scoreOf(t, s, u, b), 100) || !eq(scoreOf(t, s, u, c), 60) {
		t.Errorf("po zmianie statusu: a=%v b=%v c=%v", scoreOf(t, s, u, a), scoreOf(t, s, u, b), scoreOf(t, s, u, c))
	}

	// Usunięcie z biblioteki też przelicza.
	if err := s.DeleteEntry(ctx, u, b); err != nil {
		t.Fatal(err)
	}
	if !eq(scoreOf(t, s, u, c), 100) {
		t.Errorf("po usunięciu: c=%v", scoreOf(t, s, u, c))
	}
}

func TestSetRankingValidation(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	other := seedUser(t, s, "ewa")
	ids := seedCompleted(t, s, u, "A", "B")

	g := seedGame(t, s, 5000, "W trakcie")
	inProgress, _ := s.AddToLibrary(ctx, u, g.ID, StatusInProgress, "")
	g2 := seedGame(t, s, 5001, "Cudza")
	foreign, _ := s.AddToLibrary(ctx, other, g2.ID, StatusCompleted, "")

	tests := []struct {
		name string
		ids  []int64
	}{
		{"duplikat", []int64{ids[0], ids[0]}},
		{"gra nieukończona", []int64{ids[0], inProgress.ID}},
		{"cudza gra", []int64{ids[0], foreign.ID}},
		{"nieistniejąca", []int64{999999}},
	}
	for _, tt := range tests {
		if _, err := s.SetRanking(ctx, u, ranking.Audio, tt.ids); !errors.Is(err, ErrInvalidRanking) {
			t.Errorf("%s: err = %v", tt.name, err)
		}
	}
	// Nic nie zostało zapisane.
	if got, _ := s.Ranking(ctx, u, ranking.Audio); len(got) != 0 {
		t.Errorf("odrzucony ranking zapisał %d wierszy", len(got))
	}
	if _, _, err := s.EntryPlaces(ctx, other, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("EntryPlaces cudzego wpisu: err = %v", err)
	}
}
