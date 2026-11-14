package igdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// PopularTTL to czas życia cache „Popularne teraz” w pamięci.
	PopularTTL    = 6 * time.Hour
	popularWindow = 30 * 24 * time.Hour
)

// Popular zwraca do 12 premier z ostatnich 30 dni z ocenami, posortowanych
// po ocenie. Wynik jest wspólny dla wszystkich użytkowników i trzymany w pamięci
// przez 6 godzin; błędy nie trafiają do cache.
func (c *Client) Popular(ctx context.Context) ([]Game, error) {
	c.popMu.Lock()
	defer c.popMu.Unlock()
	now := c.now()
	if c.popular != nil && now.Before(c.popularExpires) {
		return c.popular, nil
	}
	body := fmt.Sprintf("%s; where first_release_date >= %d & first_release_date <= %d & rating_count > 0; sort rating desc; limit 12;",
		gameFields, now.Add(-popularWindow).Unix(), now.Unix())
	gs, err := c.games(ctx, body)
	if err != nil {
		return nil, err
	}
	c.popular, c.popularExpires = gs, now.Add(PopularTTL)
	return gs, nil
}

// Similar zwraca gry podobne (similar_games z IGDB) do każdej z podanych gier.
func (c *Client) Similar(ctx context.Context, ids []int64) (map[int64][]Game, error) {
	out := map[int64][]Game{}
	if len(ids) == 0 {
		return out, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	// Te same pola co przy wyszukiwaniu, ale o poziom głębiej (similar_games.*).
	fields := strings.Split(strings.TrimPrefix(gameFields, "fields "), ",")
	for i, f := range fields {
		fields[i] = "similar_games." + f
	}
	body := fmt.Sprintf("fields id,%s; where id = (%s); limit %d;", strings.Join(fields, ","), strings.Join(parts, ","), len(ids))

	var raw []struct {
		ID           int64     `json:"id"`
		SimilarGames []rawGame `json:"similar_games"`
	}
	if err := c.query(ctx, "games", body, &raw); err != nil {
		return nil, err
	}
	for _, r := range raw {
		for _, s := range r.SimilarGames {
			out[r.ID] = append(out[r.ID], s.toGame())
		}
	}
	return out, nil
}
