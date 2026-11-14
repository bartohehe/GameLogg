package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// MediaSource to źródło metadanych (enum media_source).
type MediaSource string

const (
	SourceTMDB  MediaSource = "tmdb"
	SourceJikan MediaSource = "jikan"
)

// MediaType to rodzaj tytułu (enum media_type).
type MediaType string

const (
	TypeFilm   MediaType = "film"
	TypeSerial MediaType = "serial"
	TypeAnime  MediaType = "anime"
)

// Valid mówi, czy rodzaj istnieje.
func (t MediaType) Valid() bool { return t == TypeFilm || t == TypeSerial || t == TypeAnime }

// MediaStatus to status tytułu (enum media_status).
type MediaStatus string

const (
	MediaBacklog  MediaStatus = "backlog"
	MediaWatching MediaStatus = "watching"
	MediaWatched  MediaStatus = "watched"
)

// Valid mówi, czy status istnieje.
func (s MediaStatus) Valid() bool {
	return s == MediaBacklog || s == MediaWatching || s == MediaWatched
}

var (
	// ErrNotEpisodic: filmy nie mają odcinków.
	ErrNotEpisodic = errors.New("film nie ma odcinków")
	// ErrAllWatched: obejrzano już wszystkie znane odcinki.
	ErrAllWatched = errors.New("wszystkie odcinki są już obejrzane")
)

// Media to film, serial albo anime na liście użytkownika.
type Media struct {
	ID              int64
	UserID          int64
	Source          MediaSource
	ExternalID      string
	Type            MediaType
	Title           string
	Year            *int
	Creator         string
	Genres          []string
	CoverURL        string
	RuntimeMin      *int
	Episodes        *int
	WatchedEpisodes *int
	Status          MediaStatus
	Score           *int
	Note            string
	SpineColor      string
	SpineAccent     string
	SpineInk        string
	AddedAt         time.Time
	UpdatedAt       time.Time
}

const mediaColumns = `id, user_id, source, external_id, type, title, year, creator, genres, coalesce(cover_url, ''),
	runtime_min, episodes, watched_episodes, status, score, note, spine_color, spine_accent, spine_ink, added_at, updated_at`

func scanMedia(row pgx.Row) (Media, error) {
	var m Media
	err := row.Scan(&m.ID, &m.UserID, &m.Source, &m.ExternalID, &m.Type, &m.Title, &m.Year, &m.Creator, &m.Genres,
		&m.CoverURL, &m.RuntimeMin, &m.Episodes, &m.WatchedEpisodes, &m.Status, &m.Score, &m.Note,
		&m.SpineColor, &m.SpineAccent, &m.SpineInk, &m.AddedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Media{}, ErrNotFound
	}
	return m, err
}

// AddMedia dodaje tytuł. Seriale i anime oznaczone jako obejrzane mają od razu
// wszystkie znane odcinki, a oglądane zaczynają od zera.
func (s *Store) AddMedia(ctx context.Context, m Media) (Media, error) {
	if m.Genres == nil {
		m.Genres = []string{}
	}
	if m.Type != TypeFilm {
		switch m.Status {
		case MediaWatched:
			m.WatchedEpisodes = m.Episodes
		case MediaWatching:
			zero := 0
			m.WatchedEpisodes = &zero
		}
	}
	out, err := scanMedia(s.pool.QueryRow(ctx, `
		INSERT INTO user_media (user_id, source, external_id, type, title, year, creator, genres, cover_url,
		                        runtime_min, episodes, watched_episodes, status, spine_color, spine_accent, spine_ink)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING `+mediaColumns,
		m.UserID, m.Source, m.ExternalID, m.Type, m.Title, m.Year, m.Creator, m.Genres, nullIfEmpty(m.CoverURL),
		m.RuntimeMin, m.Episodes, m.WatchedEpisodes, m.Status, m.SpineColor, m.SpineAccent, m.SpineInk))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Media{}, ErrAlreadyInLibrary
		}
		return Media{}, fmt.Errorf("add media: %w", err)
	}
	return out, nil
}

// MediaList zwraca tytuły użytkownika; pusty typ albo status = wszystkie.
// Kolejność: najpierw z oceną (od najwyższej), potem od ostatnio dodanych.
func (s *Store) MediaList(ctx context.Context, userID int64, typ MediaType, status MediaStatus) ([]Media, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+mediaColumns+` FROM user_media
		WHERE user_id = $1 AND ($2 = '' OR type::text = $2) AND ($3 = '' OR status::text = $3)
		ORDER BY score DESC NULLS LAST, added_at DESC, id DESC`, userID, string(typ), string(status))
	if err != nil {
		return nil, fmt.Errorf("media list: %w", err)
	}
	defer rows.Close()
	out := []Media{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MediaItem zwraca jeden tytuł użytkownika.
func (s *Store) MediaItem(ctx context.Context, userID, id int64) (Media, error) {
	return scanMedia(s.pool.QueryRow(ctx, `SELECT `+mediaColumns+` FROM user_media WHERE user_id = $1 AND id = $2`, userID, id))
}

// MediaRef to skrót tytułu dla wyników wyszukiwania.
type MediaRef struct {
	ID     int64
	Status MediaStatus
}

// MediaRefs zwraca tytuły użytkownika z danego źródła po zewnętrznych identyfikatorach.
func (s *Store) MediaRefs(ctx context.Context, userID int64, source MediaSource, externalIDs []string) (map[string]MediaRef, error) {
	out := map[string]MediaRef{}
	if len(externalIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT external_id, id, status FROM user_media WHERE user_id = $1 AND source = $2 AND external_id = ANY($3)`,
		userID, source, externalIDs)
	if err != nil {
		return nil, fmt.Errorf("media refs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ext string
		var r MediaRef
		if err := rows.Scan(&ext, &r.ID, &r.Status); err != nil {
			return nil, err
		}
		out[ext] = r
	}
	return out, rows.Err()
}

// MediaPatch to zmiana tytułu; nil = bez zmian. ScoreSet z Score == nil czyści ocenę.
type MediaPatch struct {
	Status          *MediaStatus
	ScoreSet        bool
	Score           *int
	WatchedEpisodes *int
	Note            *string
}

// UpdateMedia zmienia tytuł. Oznaczenie serialu albo anime jako obejrzanego
// uzupełnia licznik do wszystkich znanych odcinków; przejście na „oglądam”
// zaczyna licznik od zera, jeśli go nie było.
func (s *Store) UpdateMedia(ctx context.Context, userID, id int64, p MediaPatch) (Media, error) {
	var out Media
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := scanMedia(tx.QueryRow(ctx,
			`SELECT `+mediaColumns+` FROM user_media WHERE user_id = $1 AND id = $2 FOR UPDATE`, userID, id))
		if err != nil {
			return err
		}
		next := cur
		if p.Status != nil {
			next.Status = *p.Status
			if cur.Type != TypeFilm {
				switch {
				case next.Status == MediaWatched && next.Episodes != nil:
					next.WatchedEpisodes = next.Episodes
				case next.Status == MediaWatching && next.WatchedEpisodes == nil:
					zero := 0
					next.WatchedEpisodes = &zero
				}
			}
		}
		if p.WatchedEpisodes != nil {
			if cur.Type == TypeFilm {
				return ErrNotEpisodic
			}
			next.WatchedEpisodes = p.WatchedEpisodes
		}
		if p.ScoreSet {
			next.Score = p.Score
		}
		if p.Note != nil {
			next.Note = *p.Note
		}
		out, err = scanMedia(tx.QueryRow(ctx, `
			UPDATE user_media SET status = $3, watched_episodes = $4, score = $5, note = $6, updated_at = now()
			WHERE user_id = $1 AND id = $2 RETURNING `+mediaColumns,
			userID, id, next.Status, next.WatchedEpisodes, next.Score, next.Note))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotEpisodic) {
			return Media{}, err
		}
		return Media{}, fmt.Errorf("update media: %w", err)
	}
	return out, nil
}

// IncrementEpisode zalicza kolejny obejrzany odcinek. Tytuł z listy „do
// obejrzenia” przechodzi do „oglądam”, a ostatni odcinek kończy go jako obejrzany.
func (s *Store) IncrementEpisode(ctx context.Context, userID, id int64) (Media, error) {
	var out Media
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		cur, err := scanMedia(tx.QueryRow(ctx,
			`SELECT `+mediaColumns+` FROM user_media WHERE user_id = $1 AND id = $2 FOR UPDATE`, userID, id))
		if err != nil {
			return err
		}
		if cur.Type == TypeFilm {
			return ErrNotEpisodic
		}
		watched := 1
		if cur.WatchedEpisodes != nil {
			watched = *cur.WatchedEpisodes + 1
		}
		if cur.Episodes != nil && watched > *cur.Episodes {
			return ErrAllWatched
		}
		status := MediaWatching
		if cur.Episodes != nil && watched == *cur.Episodes {
			status = MediaWatched
		}
		out, err = scanMedia(tx.QueryRow(ctx, `
			UPDATE user_media SET watched_episodes = $3, status = $4, updated_at = now()
			WHERE user_id = $1 AND id = $2 RETURNING `+mediaColumns, userID, id, watched, status))
		return err
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrNotEpisodic) || errors.Is(err, ErrAllWatched) {
			return Media{}, err
		}
		return Media{}, fmt.Errorf("increment episode: %w", err)
	}
	return out, nil
}

// DeleteMedia usuwa tytuł z listy.
func (s *Store) DeleteMedia(ctx context.Context, userID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM user_media WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return fmt.Errorf("delete media: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
