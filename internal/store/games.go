package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Game to wiersz cache metadanych z IGDB.
type Game struct {
	ID          int64
	IGDBID      int64
	Title       string
	CoverURL    string
	ReleaseDate *time.Time
	Developer   string
	Genres      []string
	Platforms   []string
	IGDBRating  *int
	Summary     string
	SpineColor  string
	SpineAccent string
	SpineInk    string
	FetchedAt   time.Time
}

const gameColumns = `g.id, g.igdb_id, g.title, coalesce(g.cover_url, ''), g.release_date, coalesce(g.developer, ''),
	g.genres, g.platforms, g.igdb_rating, coalesce(g.summary, ''), g.spine_color, g.spine_accent, g.spine_ink, g.fetched_at`

func gameDest(g *Game) []any {
	return []any{&g.ID, &g.IGDBID, &g.Title, &g.CoverURL, &g.ReleaseDate, &g.Developer,
		&g.Genres, &g.Platforms, &g.IGDBRating, &g.Summary, &g.SpineColor, &g.SpineAccent, &g.SpineInk, &g.FetchedAt}
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GameByIGDBID zwraca grę z cache.
func (s *Store) GameByIGDBID(ctx context.Context, igdbID int64) (Game, error) {
	var g Game
	err := s.pool.QueryRow(ctx, `SELECT `+gameColumns+` FROM games g WHERE g.igdb_id = $1`, igdbID).Scan(gameDest(&g)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Game{}, ErrNotFound
	}
	return g, err
}

// UpsertGame zapisuje albo odświeża metadane gry.
func (s *Store) UpsertGame(ctx context.Context, g Game) (Game, error) {
	if g.Genres == nil {
		g.Genres = []string{}
	}
	if g.Platforms == nil {
		g.Platforms = []string{}
	}
	var out Game
	err := s.pool.QueryRow(ctx, `
		INSERT INTO games AS g (igdb_id, title, cover_url, release_date, developer, genres, platforms,
		                        igdb_rating, summary, spine_color, spine_accent, spine_ink, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (igdb_id) DO UPDATE SET
			title = excluded.title, cover_url = excluded.cover_url, release_date = excluded.release_date,
			developer = excluded.developer, genres = excluded.genres, platforms = excluded.platforms,
			igdb_rating = excluded.igdb_rating, summary = excluded.summary,
			spine_color = excluded.spine_color, spine_accent = excluded.spine_accent,
			spine_ink = excluded.spine_ink, fetched_at = now()
		RETURNING `+gameColumns,
		g.IGDBID, g.Title, nullIfEmpty(g.CoverURL), g.ReleaseDate, nullIfEmpty(g.Developer), g.Genres, g.Platforms,
		g.IGDBRating, nullIfEmpty(g.Summary), g.SpineColor, g.SpineAccent, g.SpineInk,
	).Scan(gameDest(&out)...)
	if err != nil {
		return Game{}, fmt.Errorf("upsert game: %w", err)
	}
	return out, nil
}
