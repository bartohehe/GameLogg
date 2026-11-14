package igdb

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPopularQueryAndCache(t *testing.T) {
	failNext := false
	f := &fakeIGDB{response: eldenJSON, apiStatus: func(int32) int {
		if failNext {
			return http.StatusServiceUnavailable
		}
		return http.StatusOK
	}}
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	c := newTestClient(f.server(t), func() time.Time { return now })
	ctx := context.Background()

	gs, err := c.Popular(ctx)
	if err != nil || len(gs) != 1 {
		t.Fatalf("Popular = %v, %v", gs, err)
	}
	body := f.lastBody.Load().(string)
	from, to := now.Add(-30*24*time.Hour).Unix(), now.Unix()
	wantWhere := "where first_release_date >= " + itoa64(from) + " & first_release_date <= " + itoa64(to) + " & rating_count > 0; sort rating desc; limit 12;"
	if !strings.Contains(body, wantWhere) {
		t.Errorf("zapytanie = %s\noczekiwano fragmentu %s", body, wantWhere)
	}

	// W ciągu 6 godzin bez ponownego zapytania.
	now = now.Add(5 * time.Hour)
	if _, err := c.Popular(ctx); err != nil {
		t.Fatal(err)
	}
	if n := f.apiCalls.Load(); n != 1 {
		t.Errorf("zapytań = %d, oczekiwano 1 (cache)", n)
	}

	// Po 6 godzinach cache wygasa; błąd nie trafia do cache.
	now = now.Add(2 * time.Hour)
	failNext = true
	if _, err := c.Popular(ctx); !errors.Is(err, ErrUnavailable) {
		t.Errorf("err = %v", err)
	}
	failNext = false
	if _, err := c.Popular(ctx); err != nil {
		t.Errorf("po błędzie: %v", err)
	}
	if n := f.apiCalls.Load(); n != 3 {
		t.Errorf("zapytań = %d, oczekiwano 3", n)
	}
}

func TestSimilar(t *testing.T) {
	f := &fakeIGDB{response: `[
	  {"id": 1, "similar_games": [{"id": 11, "name": "Lies of P", "cover": {"image_id": "abc"}, "platforms": [{"abbreviation": "PC"}]}, {"id": 12, "name": "Sekiro"}]},
	  {"id": 2}
	]`}
	c := newTestClient(f.server(t), time.Now)
	got, err := c.Similar(context.Background(), []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(got[1]) != 2 || got[1][0].Title != "Lies of P" || got[1][0].Platforms[0] != "PC" || !strings.HasSuffix(got[1][0].CoverURL, "/abc.jpg") {
		t.Errorf("podobne do 1 = %+v", got[1])
	}
	if len(got[2]) != 0 {
		t.Errorf("podobne do 2 = %+v", got[2])
	}
	body := f.lastBody.Load().(string)
	if !strings.HasPrefix(body, "fields id,similar_games.id,similar_games.name,similar_games.cover.image_id,") ||
		!strings.HasSuffix(body, "where id = (1,2); limit 2;") {
		t.Errorf("zapytanie = %s", body)
	}

	if m, err := c.Similar(context.Background(), nil); err != nil || len(m) != 0 {
		t.Errorf("pusta lista: %v, %v", m, err)
	}
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
