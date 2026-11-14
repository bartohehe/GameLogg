package store

import (
	"context"
	"testing"
	"time"

	"github.com/bartohehe/GameLogg/internal/ranking"
)

func TestStats(t *testing.T) {
	s := migratedStoreWithGames(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	ids := seedCompleted(t, s, u, "A", "B", "C")
	if _, err := s.SetRanking(ctx, u, ranking.Gameplay, ids); err != nil { // 100, 80, 60
		t.Fatal(err)
	}
	g := seedGame(t, s, 9000, "Porzucona")
	if _, err := s.AddToLibrary(ctx, u, g.ID, StatusAbandoned, ""); err != nil {
		t.Fatal(err)
	}
	g2 := seedGame(t, s, 9001, "Planowana")
	if _, err := s.AddToLibrary(ctx, u, g2.ID, StatusPlanned, ""); err != nil {
		t.Fatal(err)
	}
	// C ukończona w zeszłym roku: nie liczy się do bieżącego roku.
	thisYear := time.Now().UTC().Year()
	if _, err := s.pool.Exec(ctx, `UPDATE user_games SET finished_at = make_timestamptz($2, 6, 1, 0, 0, 0, 'UTC') WHERE id = $1`,
		ids[2], thisYear-1); err != nil {
		t.Fatal(err)
	}

	st, err := s.Stats(ctx, u, thisYear)
	if err != nil {
		t.Fatal(err)
	}
	if st.Total != 5 || st.Counts[StatusCompleted] != 3 || st.Counts[StatusAbandoned] != 1 || st.Counts[StatusOnHold] != 0 {
		t.Errorf("liczby = %+v, razem %d", st.Counts, st.Total)
	}
	if st.AverageScore == nil || *st.AverageScore != 80 {
		t.Errorf("średnia = %v", st.AverageScore)
	}
	if len(st.Top) != 3 || st.Top[0].Game.Title != "A" {
		t.Errorf("top = %d", len(st.Top))
	}
	y := st.Year
	if y.Completed != 2 || y.Abandoned != 1 || y.AverageScore == nil || *y.AverageScore != 90 || len(y.Top) != 2 {
		t.Errorf("rok = %+v (średnia %v)", y, y.AverageScore)
	}

	empty, err := s.Stats(ctx, seedUser(t, s, "ewa"), thisYear)
	if err != nil || empty.Total != 0 || empty.AverageScore != nil || len(empty.Top) != 0 || len(empty.Counts) != 6 {
		t.Errorf("pusta biblioteka: %+v, %v", empty, err)
	}
}

func TestTranslationCache(t *testing.T) {
	s := migratedStore(t)
	ctx := context.Background()
	if _, err := s.pool.Exec(ctx, `TRUNCATE translation_cache`); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Translation(ctx, "h", "pl", time.Now().Add(-time.Hour)); ok || err != nil {
		t.Errorf("pusty cache: %v %v", ok, err)
	}
	if err := s.SaveTranslation(ctx, "h", "pl", "Powstań, Zmatowiały."); err != nil {
		t.Fatal(err)
	}
	if text, ok, _ := s.Translation(ctx, "h", "pl", time.Now().Add(-time.Hour)); !ok || text != "Powstań, Zmatowiały." {
		t.Errorf("odczyt = %q %v", text, ok)
	}
	// Wpis starszy niż granica traktujemy jak brak.
	if _, ok, _ := s.Translation(ctx, "h", "pl", time.Now().Add(time.Hour)); ok {
		t.Error("stary wpis zwrócony")
	}
	if err := s.SaveTranslation(ctx, "h", "pl", "Nowe."); err != nil {
		t.Fatal(err)
	}
	if text, _, _ := s.Translation(ctx, "h", "pl", time.Now().Add(-time.Hour)); text != "Nowe." {
		t.Errorf("po nadpisaniu = %q", text)
	}
}
