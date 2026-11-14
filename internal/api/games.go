package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bartohehe/gamelogg/internal/igdb"
	"github.com/bartohehe/gamelogg/internal/spine"
	"github.com/bartohehe/gamelogg/internal/store"
)

// GameSource to źródło metadanych gier (klient IGDB albo atrapa w testach).
type GameSource interface {
	Search(ctx context.Context, q string) ([]igdb.Game, error)
	Game(ctx context.Context, id int64) (igdb.Game, error)
	Popular(ctx context.Context) ([]igdb.Game, error)
	Similar(ctx context.Context, ids []int64) (map[int64][]igdb.Game, error)
}

// SpineSource liczy kolory grzbietu z okładki.
type SpineSource interface {
	FromCover(ctx context.Context, coverURL, title string) (spine.Colors, error)
}

// Metadane z cache odświeżamy z IGDB po takim czasie.
const gameCacheTTL = 30 * 24 * time.Hour

const (
	msgIGDBNotConfigured = "Wyszukiwanie gier nie zadziała: brak IGDB_CLIENT_ID i IGDB_CLIENT_SECRET w .env. " +
		"Dopisz oba klucze i uruchom ponownie kontener app."
	msgIGDBUnavailable    = "IGDB nie odpowiada. Spróbuj za chwilę."
	msgIGDBBadCredentials = "IGDB odrzuciło klucze z .env. Sprawdź IGDB_CLIENT_ID i IGDB_CLIENT_SECRET " +
		"w Twitch Developer Console i uruchom ponownie kontener app."
	msgGameNotFound = "IGDB nie zna tej gry. Sprawdź adres albo wyszukaj ją ponownie."
)

func (s *server) writeIGDBError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, igdb.ErrNotFound):
		writeError(w, http.StatusNotFound, "game_not_found", msgGameNotFound)
	case errors.Is(err, igdb.ErrBadCredentials):
		s.log.WarnContext(r.Context(), "igdb rejected credentials")
		writeError(w, http.StatusBadGateway, "igdb_bad_credentials", msgIGDBBadCredentials)
	case errors.Is(err, igdb.ErrUnavailable), errors.Is(err, context.DeadlineExceeded):
		s.log.WarnContext(r.Context(), "igdb unavailable", "err", err)
		writeError(w, http.StatusBadGateway, "igdb_unavailable", msgIGDBUnavailable)
	default:
		writeInternal(w, r, s.log, err)
	}
}

func (s *server) igdbNotConfigured(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "igdb_not_configured", msgIGDBNotConfigured)
}

type searchResultDTO struct {
	gameDTO
	Entry *entryRefDTO `json:"entry"` // null, gdy gry nie ma na półce
}

func (s *server) searchGames(w http.ResponseWriter, r *http.Request) {
	if s.games == nil {
		s.igdbNotConfigured(w)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		writeJSON(w, http.StatusOK, map[string]any{"results": []searchResultDTO{}})
		return
	}
	found, err := s.games.Search(r.Context(), q)
	if err != nil {
		s.writeIGDBError(w, r, err)
		return
	}
	ids := make([]int64, len(found))
	for i, g := range found {
		ids[i] = g.ID
	}
	owned, err := s.lib.EntriesByIGDBIDs(r.Context(), userIDFrom(r.Context()), ids)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	out := make([]searchResultDTO, len(found))
	for i, g := range found {
		out[i] = searchResultDTO{gameDTO: gameFromIGDB(g)}
		if ref, ok := owned[g.ID]; ok {
			out[i].Entry = &entryRefDTO{ID: ref.ID, Status: ref.Status}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

type gameDetailsDTO struct {
	gameDTO
	Summary string `json:"summary"`
	// Polskie tłumaczenie opisu; null, gdy wyłączone albo się nie udało.
	SummaryTranslated *string   `json:"summaryTranslated"`
	Entry             *entryDTO `json:"entry"`
}

func (s *server) gameDetails(w http.ResponseWriter, r *http.Request) {
	id, ok := igdb.ParseID(chi.URLParam(r, "igdbId"))
	if !ok {
		writeError(w, http.StatusNotFound, "game_not_found", msgGameNotFound)
		return
	}
	ctx := r.Context()
	var out gameDetailsDTO

	cached, err := s.lib.GameByIGDBID(ctx, id)
	switch {
	case err == nil:
		out.gameDTO, out.Summary = gameFromStore(cached), cached.Summary
	case errors.Is(err, store.ErrNotFound):
		if s.games == nil {
			s.igdbNotConfigured(w)
			return
		}
		g, err := s.games.Game(ctx, id)
		if err != nil {
			s.writeIGDBError(w, r, err)
			return
		}
		out.gameDTO, out.Summary = gameFromIGDB(g), g.Summary
	default:
		writeInternal(w, r, s.log, err)
		return
	}

	out.SummaryTranslated = s.translateSummary(ctx, out.Summary)

	e, err := s.lib.EntryByIGDBID(ctx, userIDFrom(ctx), id)
	switch {
	case err == nil:
		dto := entryFromStore(e)
		out.Entry = &dto
	case !errors.Is(err, store.ErrNotFound):
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ensureGame zwraca grę z cache, a gdy jej brak albo jest stara, pobiera ją
// z IGDB i zapisuje razem z kolorami grzbietu. Przy niedostępnym IGDB
// wystarcza stara kopia z cache.
func (s *server) ensureGame(ctx context.Context, igdbID int64) (store.Game, error) {
	cached, err := s.lib.GameByIGDBID(ctx, igdbID)
	hasCache := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return store.Game{}, err
	}
	if hasCache && time.Since(cached.FetchedAt) < gameCacheTTL {
		return cached, nil
	}
	if s.games == nil {
		if hasCache {
			return cached, nil
		}
		return store.Game{}, errIGDBNotConfigured
	}

	g, err := s.games.Game(ctx, igdbID)
	if err != nil {
		if hasCache && !errors.Is(err, igdb.ErrNotFound) {
			s.log.WarnContext(ctx, "igdb refresh failed, using cached game", "igdb_id", igdbID, "err", err)
			return cached, nil
		}
		return store.Game{}, err
	}

	row := store.Game{
		IGDBID: g.ID, Title: g.Title, CoverURL: g.CoverURL, ReleaseDate: g.ReleaseDate, Developer: g.Developer,
		Genres: g.Genres, Platforms: g.Platforms, IGDBRating: g.Rating, Summary: g.Summary,
	}
	if hasCache && cached.CoverURL == g.CoverURL {
		row.SpineColor, row.SpineAccent, row.SpineInk = cached.SpineColor, cached.SpineAccent, cached.SpineInk
	} else {
		colors, err := s.spines.FromCover(ctx, g.CoverURL, g.Title)
		if err != nil {
			s.log.WarnContext(ctx, "spine colors from title hash", "igdb_id", igdbID, "err", err)
		}
		row.SpineColor, row.SpineAccent, row.SpineInk = colors.Color, colors.Accent, colors.Ink
	}
	return s.lib.UpsertGame(ctx, row)
}

var errIGDBNotConfigured = errors.New("igdb not configured")
