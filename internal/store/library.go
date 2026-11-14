package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Status to status gry na półce (enum game_status).
type Status string

const (
	StatusPlanned    Status = "planned"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusOnHold     Status = "on_hold"
	StatusWishlist   Status = "wishlist"
	StatusAbandoned  Status = "abandoned"
)

// Statuses w kolejności półek w bibliotece.
var Statuses = []Status{StatusInProgress, StatusPlanned, StatusCompleted, StatusOnHold, StatusWishlist, StatusAbandoned}

// Valid mówi, czy status istnieje.
func (s Status) Valid() bool {
	for _, v := range Statuses {
		if s == v {
			return true
		}
	}
	return false
}

// ErrAlreadyInLibrary: użytkownik ma już tę grę na półce.
var ErrAlreadyInLibrary = errors.New("gra jest już w bibliotece")

// LibraryEntry to gra użytkownika razem z metadanymi z cache.
type LibraryEntry struct {
	ID             int64
	UserID         int64
	Status         Status
	Platform       string
	Score          *int
	Note           string
	CompletedCount int
	FinishedAt     *time.Time
	AddedAt        time.Time
	UpdatedAt      time.Time
	Game           Game
}

const entryColumns = `ug.id, ug.user_id, ug.status, ug.platform, ug.score, ug.note, ug.completed_count,
	ug.finished_at, ug.added_at, ug.updated_at, ` + gameColumns

const entryFrom = ` FROM user_games ug JOIN games g ON g.id = ug.game_id `

func scanEntry(row pgx.Row) (LibraryEntry, error) {
	var e LibraryEntry
	dest := append([]any{&e.ID, &e.UserID, &e.Status, &e.Platform, &e.Score, &e.Note, &e.CompletedCount,
		&e.FinishedAt, &e.AddedAt, &e.UpdatedAt}, gameDest(&e.Game)...)
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return LibraryEntry{}, ErrNotFound
	}
	return e, err
}

// Sortowania biblioteki. Klucz to parametr ?sort=, wartość to bezpieczny ORDER BY.
var librarySorts = map[string]string{
	"score": "ug.score DESC NULLS LAST, lower(g.title), ug.id",
	"title": "lower(g.title), g.title, ug.id",
	"added": "ug.added_at DESC, ug.id DESC",
}

// ValidSort mówi, czy sortowanie istnieje.
func ValidSort(s string) bool {
	_, ok := librarySorts[s]
	return ok
}

// Library zwraca gry użytkownika. Pusty status = wszystkie, pusty sort = "score".
func (s *Store) Library(ctx context.Context, userID int64, status Status, sort string) ([]LibraryEntry, error) {
	order, ok := librarySorts[sort]
	if !ok {
		order = librarySorts["score"]
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+entryColumns+entryFrom+`
		 WHERE ug.user_id = $1 AND ($2 = '' OR ug.status::text = $2)
		 ORDER BY `+order, userID, string(status))
	if err != nil {
		return nil, fmt.Errorf("library: %w", err)
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

// Entry zwraca jeden wpis użytkownika.
func (s *Store) Entry(ctx context.Context, userID, id int64) (LibraryEntry, error) {
	return scanEntry(s.pool.QueryRow(ctx,
		`SELECT `+entryColumns+entryFrom+` WHERE ug.user_id = $1 AND ug.id = $2`, userID, id))
}

// EntryByIGDBID zwraca wpis użytkownika dla gry z IGDB.
func (s *Store) EntryByIGDBID(ctx context.Context, userID, igdbID int64) (LibraryEntry, error) {
	return scanEntry(s.pool.QueryRow(ctx,
		`SELECT `+entryColumns+entryFrom+` WHERE ug.user_id = $1 AND g.igdb_id = $2`, userID, igdbID))
}

// EntryRef to skrót wpisu: wystarcza wynikom wyszukiwania („Na półce: …”).
type EntryRef struct {
	ID     int64
	Status Status
}

// EntriesByIGDBIDs zwraca wpisy użytkownika dla podanych gier IGDB.
func (s *Store) EntriesByIGDBIDs(ctx context.Context, userID int64, igdbIDs []int64) (map[int64]EntryRef, error) {
	out := map[int64]EntryRef{}
	if len(igdbIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT g.igdb_id, ug.id, ug.status FROM user_games ug JOIN games g ON g.id = ug.game_id
		 WHERE ug.user_id = $1 AND g.igdb_id = ANY($2)`, userID, igdbIDs)
	if err != nil {
		return nil, fmt.Errorf("entries by igdb: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var igdbID int64
		var r EntryRef
		if err := rows.Scan(&igdbID, &r.ID, &r.Status); err != nil {
			return nil, err
		}
		out[igdbID] = r
	}
	return out, rows.Err()
}

// AddToLibrary stawia grę na półce. Gra dodana od razu jako ukończona liczy
// się jako pierwsze ukończenie.
func (s *Store) AddToLibrary(ctx context.Context, userID, gameID int64, status Status, platform string) (LibraryEntry, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO user_games (user_id, game_id, status, platform, completed_count, finished_at)
		VALUES ($1, $2, $3::game_status, $4,
		        CASE WHEN $3::game_status = 'completed' THEN 1 ELSE 0 END,
		        CASE WHEN $3::game_status IN ('completed', 'abandoned') THEN now() END)
		RETURNING id`, userID, gameID, status, platform).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return LibraryEntry{}, ErrAlreadyInLibrary
		}
		return LibraryEntry{}, fmt.Errorf("add to library: %w", err)
	}
	return s.Entry(ctx, userID, id)
}

// EntryPatch to zmiana wpisu; nil = bez zmian.
type EntryPatch struct {
	Status   *Status
	Platform *string
	Note     *string
}

// UpdateEntry zmienia wpis. Zmiana statusu:
//   - na completed z innego zwiększa completed_count;
//   - na completed albo abandoned ustawia finished_at, na inny je czyści;
//   - z completed na inny kasuje ocenę i usuwa grę ze wszystkich rankingów.
func (s *Store) UpdateEntry(ctx context.Context, userID, id int64, p EntryPatch) (LibraryEntry, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var old Status
		err := tx.QueryRow(ctx, `SELECT status FROM user_games WHERE id = $1 AND user_id = $2 FOR UPDATE`,
			id, userID).Scan(&old)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		if p.Status != nil && *p.Status != old {
			_, err = tx.Exec(ctx, `
				UPDATE user_games SET
					status = $3::game_status,
					completed_count = completed_count + CASE WHEN $3::game_status = 'completed' THEN 1 ELSE 0 END,
					finished_at = CASE WHEN $3::game_status IN ('completed', 'abandoned') THEN now() END,
					score = CASE WHEN $3::game_status = 'completed' THEN score END,
					updated_at = now()
				WHERE id = $1 AND user_id = $2`, id, userID, *p.Status)
			if err != nil {
				return err
			}
			if old == StatusCompleted {
				if err := removeFromRankings(ctx, tx, userID, id); err != nil {
					return err
				}
			}
		}
		if p.Platform != nil || p.Note != nil {
			_, err = tx.Exec(ctx, `
				UPDATE user_games SET
					platform = coalesce($3, platform),
					note = coalesce($4, note),
					updated_at = now()
				WHERE id = $1 AND user_id = $2`, id, userID, p.Platform, p.Note)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LibraryEntry{}, err
		}
		return LibraryEntry{}, fmt.Errorf("update entry: %w", err)
	}
	return s.Entry(ctx, userID, id)
}

// DeleteEntry zdejmuje grę z półki razem z jej miejscami w rankingach.
func (s *Store) DeleteEntry(ctx context.Context, userID, id int64) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM user_games WHERE id = $1 AND user_id = $2`, id, userID)
		if err != nil {
			return fmt.Errorf("delete entry: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		// Wiersze rankingów zniknęły kaskadowo; zostały dziury w pozycjach.
		return afterRankingChange(ctx, tx, userID)
	})
}

// removeFromRankings usuwa grę ze wszystkich kategorii użytkownika.
func removeFromRankings(ctx context.Context, tx pgx.Tx, userID, userGameID int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM category_rankings WHERE user_id = $1 AND user_game_id = $2`,
		userID, userGameID); err != nil {
		return fmt.Errorf("remove from rankings: %w", err)
	}
	return afterRankingChange(ctx, tx, userID)
}

// afterRankingChange zagęszcza pozycje (0, 1, 2, …) we wszystkich kategoriach
// użytkownika i przelicza oceny ukończonych gier: zniknięcie gry z rankingu
// zmienia n i punkty pozostałych. Unikalność pozycji jest odroczona do
// commitu, więc chwilowe duplikaty w trakcie przenumerowania są dozwolone.
func afterRankingChange(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE category_rankings cr SET position = r.rn - 1
		FROM (
			SELECT category, user_game_id,
			       row_number() OVER (PARTITION BY category ORDER BY position) AS rn
			FROM category_rankings WHERE user_id = $1
		) r
		WHERE cr.user_id = $1 AND cr.category = r.category AND cr.user_game_id = r.user_game_id
		  AND cr.position <> r.rn - 1`, userID)
	if err != nil {
		return fmt.Errorf("compact rankings: %w", err)
	}
	_, err = recomputeScores(ctx, tx, userID)
	return err
}
