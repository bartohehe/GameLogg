package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/bartohehe/GameLogg/internal/ranking"
)

// ErrInvalidRanking: lista zawiera duplikaty albo gry, które nie są ukończonymi
// grami tego użytkownika.
var ErrInvalidRanking = errors.New("ranking może zawierać tylko ukończone gry z twojej biblioteki, każdą raz")

// RankedEntry to gra na liście rankingu kategorii.
type RankedEntry struct {
	Position int // 1-based
	Entry    LibraryEntry
}

// Ranking zwraca uporządkowaną listę gier w kategorii (od najlepszej).
func (s *Store) Ranking(ctx context.Context, userID int64, cat ranking.Category) ([]RankedEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT cr.position, `+entryColumns+entryFrom+`
		 JOIN category_rankings cr ON cr.user_game_id = ug.id AND cr.user_id = ug.user_id
		 WHERE ug.user_id = $1 AND cr.category = $2
		 ORDER BY cr.position`, userID, string(cat))
	if err != nil {
		return nil, fmt.Errorf("ranking: %w", err)
	}
	defer rows.Close()
	out := []RankedEntry{}
	for rows.Next() {
		var pos int
		var e LibraryEntry
		dest := append([]any{&pos, &e.ID, &e.UserID, &e.Status, &e.Platform, &e.Score, &e.Note, &e.CompletedCount,
			&e.FinishedAt, &e.AddedAt, &e.UpdatedAt}, gameDest(&e.Game)...)
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, RankedEntry{Position: pos + 1, Entry: e})
	}
	return out, rows.Err()
}

// EntryPlaces zwraca miejsca wpisu we wszystkich kategoriach i liczbę gier
// w każdej z nich (także tam, gdzie wpisu nie ma).
func (s *Store) EntryPlaces(ctx context.Context, userID, entryID int64) (map[ranking.Category]ranking.Place, map[ranking.Category]int, error) {
	if _, err := s.Entry(ctx, userID, entryID); err != nil {
		return nil, nil, err
	}
	r, err := loadRankings(ctx, s.pool, userID)
	if err != nil {
		return nil, nil, err
	}
	sizes := map[ranking.Category]int{}
	for _, c := range ranking.Categories {
		sizes[c] = len(r[c])
	}
	return r.Places(entryID), sizes, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func loadRankings(ctx context.Context, q querier, userID int64) (ranking.Rankings, error) {
	rows, err := q.Query(ctx,
		`SELECT category, user_game_id FROM category_rankings WHERE user_id = $1 ORDER BY category, position`, userID)
	if err != nil {
		return nil, fmt.Errorf("load rankings: %w", err)
	}
	defer rows.Close()
	r := ranking.Rankings{}
	for rows.Next() {
		var c string
		var id int64
		if err := rows.Scan(&c, &id); err != nil {
			return nil, err
		}
		r[ranking.Category(c)] = append(r[ranking.Category(c)], id)
	}
	return r, rows.Err()
}

// SetRanking ustawia pełną kolejność gier w kategorii i w tej samej transakcji
// przelicza oceny wszystkich ukończonych gier użytkownika (wstawienie gry
// zmienia n i punkty pozostałych). Zwraca nowe oceny ukończonych gier.
func (s *Store) SetRanking(ctx context.Context, userID int64, cat ranking.Category, ids []int64) (map[int64]*int, error) {
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, ErrInvalidRanking
		}
		seen[id] = true
	}

	var scores map[int64]*int
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Blokada wpisów użytkownika: równoległe zmiany rankingów idą po kolei.
		if _, err := tx.Exec(ctx, `SELECT id FROM user_games WHERE user_id = $1 ORDER BY id FOR UPDATE`, userID); err != nil {
			return err
		}
		var completed int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM user_games WHERE user_id = $1 AND id = ANY($2) AND status = 'completed'`,
			userID, ids).Scan(&completed); err != nil {
			return err
		}
		if completed != len(ids) {
			return ErrInvalidRanking
		}

		if _, err := tx.Exec(ctx, `DELETE FROM category_rankings WHERE user_id = $1 AND category = $2`,
			userID, string(cat)); err != nil {
			return err
		}
		if len(ids) > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO category_rankings (user_id, category, user_game_id, position)
				SELECT $1, $2, id, ord - 1 FROM unnest($3::bigint[]) WITH ORDINALITY AS t(id, ord)`,
				userID, string(cat), ids); err != nil {
				return err
			}
		}
		var err error
		scores, err = recomputeScores(ctx, tx, userID)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrInvalidRanking) {
			return nil, err
		}
		return nil, fmt.Errorf("set ranking: %w", err)
	}
	return scores, nil
}

// recomputeScores liczy od nowa oceny wszystkich ukończonych gier użytkownika
// i zapisuje je w user_games.score (żeby sortowanie i półki nie liczyły ich w locie).
func recomputeScores(ctx context.Context, tx pgx.Tx, userID int64) (map[int64]*int, error) {
	r, err := loadRankings(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM user_games WHERE user_id = $1 AND status = 'completed'`, userID)
	if err != nil {
		return nil, err
	}
	games, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return nil, err
	}

	scores := ranking.Recompute(r, games)
	ids := make([]int64, 0, len(scores))
	vals := make([]*int, 0, len(scores))
	for id, sc := range scores {
		ids = append(ids, id)
		vals = append(vals, sc)
	}
	if len(ids) > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE user_games ug SET score = v.score
			FROM unnest($2::bigint[], $3::smallint[]) AS v(id, score)
			WHERE ug.user_id = $1 AND ug.id = v.id`, userID, ids, vals)
		if err != nil {
			return nil, fmt.Errorf("save scores: %w", err)
		}
	}
	return scores, nil
}
