package store

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
)

// Stats to podsumowanie biblioteki użytkownika.
type Stats struct {
	Counts       map[Status]int
	Total        int
	AverageScore *int
	Top          []LibraryEntry // do 8 najwyżej ocenionych

	Year YearStats
}

// YearStats to podsumowanie jednego roku kalendarzowego (po finished_at).
type YearStats struct {
	Year         int
	Completed    int
	Abandoned    int
	AverageScore *int
	Top          []LibraryEntry // do 5 najwyżej ocenionych gier ukończonych w tym roku
}

func roundAvg(v *float64) *int {
	if v == nil {
		return nil
	}
	r := int(math.Floor(*v + 0.5))
	return &r
}

// TopRated zwraca najwyżej ocenione gry (tylko z oceną).
func (s *Store) TopRated(ctx context.Context, userID int64, limit int) ([]LibraryEntry, error) {
	return s.entries(ctx,
		`SELECT `+entryColumns+entryFrom+`
		 WHERE ug.user_id = $1 AND ug.score IS NOT NULL
		 ORDER BY ug.score DESC, lower(g.title), ug.id LIMIT $2`, userID, limit)
}

func (s *Store) entries(ctx context.Context, sql string, args ...any) ([]LibraryEntry, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LibraryEntry{}
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Stats liczy statystyki biblioteki i wybranego roku. Rok wyznacza
// finished_at (moment trafienia na półkę Ukończone albo Porzucone).
func (s *Store) Stats(ctx context.Context, userID int64, year int) (Stats, error) {
	st := Stats{Counts: map[Status]int{}, Year: YearStats{Year: year}}
	for _, status := range Statuses {
		st.Counts[status] = 0
	}

	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM user_games WHERE user_id = $1 GROUP BY status`, userID)
	if err != nil {
		return Stats{}, fmt.Errorf("stats counts: %w", err)
	}
	for rows.Next() {
		var status Status
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			rows.Close()
			return Stats{}, err
		}
		st.Counts[status] = n
		st.Total += n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Stats{}, err
	}

	from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(1, 0, 0)
	var avg, yearAvg *float64
	err = s.pool.QueryRow(ctx, `
		SELECT
			avg(score) FILTER (WHERE status = 'completed'),
			count(*) FILTER (WHERE status = 'completed' AND finished_at >= $2 AND finished_at < $3),
			count(*) FILTER (WHERE status = 'abandoned' AND finished_at >= $2 AND finished_at < $3),
			avg(score) FILTER (WHERE status = 'completed' AND finished_at >= $2 AND finished_at < $3)
		FROM user_games WHERE user_id = $1`, userID, from, to,
	).Scan(&avg, &st.Year.Completed, &st.Year.Abandoned, &yearAvg)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Stats{}, fmt.Errorf("stats averages: %w", err)
	}
	st.AverageScore, st.Year.AverageScore = roundAvg(avg), roundAvg(yearAvg)

	if st.Top, err = s.TopRated(ctx, userID, 8); err != nil {
		return Stats{}, fmt.Errorf("stats top: %w", err)
	}
	st.Year.Top, err = s.entries(ctx,
		`SELECT `+entryColumns+entryFrom+`
		 WHERE ug.user_id = $1 AND ug.score IS NOT NULL AND ug.status = 'completed'
		   AND ug.finished_at >= $2 AND ug.finished_at < $3
		 ORDER BY ug.score DESC, lower(g.title), ug.id LIMIT 5`, userID, from, to)
	if err != nil {
		return Stats{}, fmt.Errorf("stats year top: %w", err)
	}
	return st, nil
}

// Translation zwraca tłumaczenie z cache, jeśli jest młodsze niż newerThan.
func (s *Store) Translation(ctx context.Context, hash, lang string, newerThan time.Time) (string, bool, error) {
	var text string
	err := s.pool.QueryRow(ctx,
		`SELECT translated FROM translation_cache WHERE source_hash = $1 AND lang = $2 AND created_at > $3`,
		hash, lang, newerThan).Scan(&text)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("translation cache: %w", err)
	}
	return text, true, nil
}

// SaveTranslation zapisuje albo odświeża tłumaczenie w cache.
func (s *Store) SaveTranslation(ctx context.Context, hash, lang, text string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO translation_cache (source_hash, lang, translated, created_at) VALUES ($1, $2, $3, now())
		ON CONFLICT (source_hash, lang) DO UPDATE SET translated = excluded.translated, created_at = now()`,
		hash, lang, text)
	if err != nil {
		return fmt.Errorf("save translation: %w", err)
	}
	return nil
}
