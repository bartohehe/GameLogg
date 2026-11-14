package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bartohehe/GameLogg/internal/jikan"
	"github.com/bartohehe/GameLogg/internal/store"
	"github.com/bartohehe/GameLogg/internal/tmdb"
)

// MovieSource to źródło filmów i seriali (TMDB). nil bez TMDB_API_KEY.
type MovieSource interface {
	Search(ctx context.Context, q string) ([]tmdb.Title, error)
	Details(ctx context.Context, kind tmdb.Kind, id int64) (tmdb.Title, error)
}

// AnimeSource to źródło anime (Jikan / MyAnimeList).
type AnimeSource interface {
	Search(ctx context.Context, q string) ([]jikan.Anime, error)
	Anime(ctx context.Context, id int64) (jikan.Anime, error)
}

// MediaStore to operacje na liście filmów, seriali i anime.
type MediaStore interface {
	AddMedia(ctx context.Context, m store.Media) (store.Media, error)
	MediaList(ctx context.Context, userID int64, typ store.MediaType, status store.MediaStatus) ([]store.Media, error)
	MediaItem(ctx context.Context, userID, id int64) (store.Media, error)
	MediaRefs(ctx context.Context, userID int64, source store.MediaSource, externalIDs []string) (map[string]store.MediaRef, error)
	UpdateMedia(ctx context.Context, userID, id int64, p store.MediaPatch) (store.Media, error)
	IncrementEpisode(ctx context.Context, userID, id int64) (store.Media, error)
	DeleteMedia(ctx context.Context, userID, id int64) error
}

const (
	msgTMDBNotConfigured = "Wyszukiwanie filmów i seriali nie zadziała: brak TMDB_API_KEY w .env. " +
		"Dopisz klucz i uruchom ponownie kontener app. Anime z MyAnimeList działa bez klucza."
	msgTMDBBadKey      = "TMDB odrzuciło klucz z .env. Sprawdź TMDB_API_KEY i uruchom ponownie kontener app."
	msgTMDBUnavailable = "TMDB nie odpowiada. Spróbuj za chwilę."
	msgJikanUnavail    = "MyAnimeList (Jikan) nie odpowiada. Spróbuj za chwilę."
	msgMediaNotFound   = "Tego tytułu nie ma na twojej liście."
	mediaSearchTimeout = 10 * time.Second
)

type mediaRefDTO struct {
	ID     int64             `json:"id"`
	Status store.MediaStatus `json:"status"`
}

type mediaResultDTO struct {
	Source     store.MediaSource `json:"source"`
	ExternalID string            `json:"externalId"`
	Type       store.MediaType   `json:"type"`
	Title      string            `json:"title"`
	Year       *int              `json:"year"`
	Creator    string            `json:"creator"`
	Genres     []string          `json:"genres"`
	CoverURL   *string           `json:"coverUrl"`
	Score      *int              `json:"score"`
	Episodes   *int              `json:"episodes"`
	RuntimeMin *int              `json:"runtimeMin"`
	Entry      *mediaRefDTO      `json:"entry"`
}

type mediaDTO struct {
	ID              int64             `json:"id"`
	Source          store.MediaSource `json:"source"`
	ExternalID      string            `json:"externalId"`
	Type            store.MediaType   `json:"type"`
	Title           string            `json:"title"`
	Year            *int              `json:"year"`
	Creator         string            `json:"creator"`
	Genres          []string          `json:"genres"`
	CoverURL        *string           `json:"coverUrl"`
	RuntimeMin      *int              `json:"runtimeMin"`
	Episodes        *int              `json:"episodes"`
	WatchedEpisodes *int              `json:"watchedEpisodes"`
	Status          store.MediaStatus `json:"status"`
	Score           *int              `json:"score"`
	Note            string            `json:"note"`
	Spine           spineDTO          `json:"spine"`
	AddedAt         time.Time         `json:"addedAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

func mediaFromStore(m store.Media) mediaDTO {
	return mediaDTO{
		ID: m.ID, Source: m.Source, ExternalID: m.ExternalID, Type: m.Type, Title: m.Title, Year: m.Year,
		Creator: m.Creator, Genres: nonNil(m.Genres), CoverURL: strPtr(m.CoverURL), RuntimeMin: m.RuntimeMin,
		Episodes: m.Episodes, WatchedEpisodes: m.WatchedEpisodes, Status: m.Status, Score: m.Score, Note: m.Note,
		Spine:   spineDTO{Color: m.SpineColor, Accent: m.SpineAccent, Ink: m.SpineInk},
		AddedAt: m.AddedAt, UpdatedAt: m.UpdatedAt,
	}
}

func kindToType(k tmdb.Kind) store.MediaType {
	if k == tmdb.TV {
		return store.TypeSerial
	}
	return store.TypeFilm
}

func fromTMDB(t tmdb.Title) mediaResultDTO {
	return mediaResultDTO{
		Source: store.SourceTMDB, ExternalID: strconv.FormatInt(t.ID, 10), Type: kindToType(t.Kind), Title: t.Title,
		Year: t.Year, Creator: t.Creator, Genres: nonNil(t.Genres), CoverURL: strPtr(t.PosterURL), Score: t.Score,
		Episodes: t.Episodes, RuntimeMin: t.RuntimeMin,
	}
}

func fromJikan(a jikan.Anime) mediaResultDTO {
	return mediaResultDTO{
		Source: store.SourceJikan, ExternalID: strconv.FormatInt(a.ID, 10), Type: store.TypeAnime, Title: a.Title,
		Year: a.Year, Creator: a.Studios, Genres: nonNil(a.Genres), CoverURL: strPtr(a.ImageURL), Score: a.Score,
		Episodes: a.Episodes, RuntimeMin: a.RuntimeMin,
	}
}

type sourceErrorDTO struct {
	Source  store.MediaSource `json:"source"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
}

func tmdbError(err error) sourceErrorDTO {
	if errors.Is(err, tmdb.ErrBadKey) {
		return sourceErrorDTO{store.SourceTMDB, "tmdb_bad_key", msgTMDBBadKey}
	}
	return sourceErrorDTO{store.SourceTMDB, "tmdb_unavailable", msgTMDBUnavailable}
}

func jikanError(error) sourceErrorDTO {
	return sourceErrorDTO{store.SourceJikan, "jikan_unavailable", msgJikanUnavail}
}

// searchMedia szuka równolegle w TMDB (filmy, seriale) i Jikan (anime).
// Błąd jednego źródła nie psuje wyników drugiego: trafia do pola errors.
func (s *server) searchMedia(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := store.MediaType(r.URL.Query().Get("type"))
	if typ != "" && !typ.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_type", "type", "Nieznany rodzaj. Dozwolone: film, serial, anime.")
		return
	}
	wantMovies := typ != store.TypeAnime
	wantAnime := typ == "" || typ == store.TypeAnime
	if wantMovies && !wantAnime && s.movies == nil {
		writeError(w, http.StatusServiceUnavailable, "tmdb_not_configured", msgTMDBNotConfigured)
		return
	}
	resp := map[string]any{"results": []mediaResultDTO{}, "tmdbConfigured": s.movies != nil, "errors": []sourceErrorDTO{}}
	if len([]rune(q)) < 2 {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), mediaSearchTimeout)
	defer cancel()
	var (
		wg               sync.WaitGroup
		movies, anime    []mediaResultDTO
		movieErr, animeE *sourceErrorDTO
		queried          int
	)
	if wantMovies && s.movies != nil {
		queried++
		wg.Add(1)
		go func() {
			defer wg.Done()
			found, err := s.movies.Search(ctx, q)
			if err != nil {
				e := tmdbError(err)
				movieErr = &e
				s.log.WarnContext(ctx, "tmdb search failed", "err", err)
				return
			}
			for _, t := range found {
				if typ == "" || kindToType(t.Kind) == typ {
					movies = append(movies, fromTMDB(t))
				}
			}
		}()
	}
	if wantAnime {
		queried++
		wg.Add(1)
		go func() {
			defer wg.Done()
			found, err := s.anime.Search(ctx, q)
			if err != nil {
				e := jikanError(err)
				animeE = &e
				s.log.WarnContext(ctx, "jikan search failed", "err", err)
				return
			}
			for _, a := range found {
				anime = append(anime, fromJikan(a))
			}
		}()
	}
	wg.Wait()

	var errs []sourceErrorDTO
	for _, e := range []*sourceErrorDTO{movieErr, animeE} {
		if e != nil {
			errs = append(errs, *e)
		}
	}
	if queried > 0 && len(errs) == queried {
		writeError(w, http.StatusBadGateway, errs[0].Code, errs[0].Message)
		return
	}

	results := append(movies, anime...)
	if err := s.attachMediaRefs(r.Context(), results); err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	if results == nil {
		results = []mediaResultDTO{}
	}
	if errs == nil {
		errs = []sourceErrorDTO{}
	}
	resp["results"], resp["errors"] = results, errs
	writeJSON(w, http.StatusOK, resp)
}

func (s *server) attachMediaRefs(ctx context.Context, results []mediaResultDTO) error {
	uid := userIDFrom(ctx)
	bySource := map[store.MediaSource][]string{}
	for _, r := range results {
		bySource[r.Source] = append(bySource[r.Source], r.ExternalID)
	}
	for src, ids := range bySource {
		refs, err := s.media.MediaRefs(ctx, uid, src, ids)
		if err != nil {
			return err
		}
		for i := range results {
			if results[i].Source != src {
				continue
			}
			if ref, ok := refs[results[i].ExternalID]; ok {
				results[i].Entry = &mediaRefDTO{ID: ref.ID, Status: ref.Status}
			}
		}
	}
	return nil
}

func (s *server) listMedia(w http.ResponseWriter, r *http.Request) {
	typ := store.MediaType(r.URL.Query().Get("type"))
	status := store.MediaStatus(r.URL.Query().Get("status"))
	if typ != "" && !typ.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_type", "type", "Nieznany rodzaj. Dozwolone: film, serial, anime.")
		return
	}
	if status != "" && !status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", "Nieznany status. Dozwolone: backlog, watching, watched.")
		return
	}
	list, err := s.media.MediaList(r.Context(), userIDFrom(r.Context()), typ, status)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	items := make([]mediaDTO, len(list))
	for i, m := range list {
		items[i] = mediaFromStore(m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type addMediaRequest struct {
	Source     store.MediaSource `json:"source"`
	ExternalID string            `json:"externalId"`
	Type       store.MediaType   `json:"type"`
	Status     store.MediaStatus `json:"status"`
}

func (s *server) addMedia(w http.ResponseWriter, r *http.Request) {
	var req addMediaRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Status == "" {
		req.Status = store.MediaBacklog
	}
	if !req.Status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", "Nieznany status. Dozwolone: backlog, watching, watched.")
		return
	}
	extID, err := strconv.ParseInt(req.ExternalID, 10, 64)
	if err != nil || extID <= 0 {
		writeFieldError(w, http.StatusBadRequest, "invalid_external_id", "externalId", "Brak identyfikatora tytułu ze źródła.")
		return
	}
	ctx := r.Context()

	// Metadane zawsze pobieramy ze źródła, a nie z ciała żądania.
	var item mediaResultDTO
	switch req.Source {
	case store.SourceTMDB:
		if req.Type != store.TypeFilm && req.Type != store.TypeSerial {
			writeFieldError(w, http.StatusBadRequest, "invalid_type", "type", "Tytuł z TMDB to film albo serial.")
			return
		}
		if s.movies == nil {
			writeError(w, http.StatusServiceUnavailable, "tmdb_not_configured", msgTMDBNotConfigured)
			return
		}
		kind := tmdb.Movie
		if req.Type == store.TypeSerial {
			kind = tmdb.TV
		}
		t, err := s.movies.Details(ctx, kind, extID)
		if errors.Is(err, tmdb.ErrNotFound) {
			writeError(w, http.StatusNotFound, "media_not_found", "TMDB nie zna tego tytułu.")
			return
		}
		if err != nil {
			e := tmdbError(err)
			writeError(w, http.StatusBadGateway, e.Code, e.Message)
			return
		}
		item = fromTMDB(t)
	case store.SourceJikan:
		a, err := s.anime.Anime(ctx, extID)
		if errors.Is(err, jikan.ErrNotFound) {
			writeError(w, http.StatusNotFound, "media_not_found", "MyAnimeList nie zna tego tytułu.")
			return
		}
		if err != nil {
			e := jikanError(err)
			writeError(w, http.StatusBadGateway, e.Code, e.Message)
			return
		}
		item = fromJikan(a)
	default:
		writeFieldError(w, http.StatusBadRequest, "invalid_source", "source", "Nieznane źródło. Dozwolone: tmdb, jikan.")
		return
	}

	cover := ""
	if item.CoverURL != nil {
		cover = *item.CoverURL
	}
	colors, err := s.spines.FromCover(ctx, cover, item.Title)
	if err != nil {
		s.log.WarnContext(ctx, "media spine colors from title hash", "title", item.Title, "err", err)
	}
	m, err := s.media.AddMedia(ctx, store.Media{
		UserID: userIDFrom(ctx), Source: item.Source, ExternalID: item.ExternalID, Type: item.Type, Title: item.Title,
		Year: item.Year, Creator: item.Creator, Genres: item.Genres, CoverURL: cover, RuntimeMin: item.RuntimeMin,
		Episodes: item.Episodes, Status: req.Status,
		SpineColor: colors.Color, SpineAccent: colors.Accent, SpineInk: colors.Ink,
	})
	if errors.Is(err, store.ErrAlreadyInLibrary) {
		writeError(w, http.StatusConflict, "already_in_list", "Ten tytuł już jest na twojej liście.")
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, mediaFromStore(m))
}

type patchMediaRequest struct {
	Status          *store.MediaStatus `json:"status"`
	Score           json.RawMessage    `json:"score"` // null czyści ocenę
	WatchedEpisodes *int               `json:"watchedEpisodes"`
	Note            *string            `json:"note"`
}

func mediaIDParam(r *http.Request) (int64, bool) {
	return entryIDParam(r)
}

func (s *server) writeMediaErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "media_not_found", msgMediaNotFound)
	case errors.Is(err, store.ErrNotEpisodic):
		writeError(w, http.StatusBadRequest, "not_episodic", "Film nie ma odcinków. Oznacz go jako obejrzany.")
	case errors.Is(err, store.ErrAllWatched):
		writeError(w, http.StatusConflict, "all_watched", "Wszystkie odcinki są już obejrzane.")
	default:
		writeInternal(w, r, s.log, err)
	}
}

func (s *server) patchMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaIDParam(r)
	if !ok {
		writeError(w, http.StatusNotFound, "media_not_found", msgMediaNotFound)
		return
	}
	var req patchMediaRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	p := store.MediaPatch{Status: req.Status, WatchedEpisodes: req.WatchedEpisodes, Note: req.Note}
	if req.Status != nil && !req.Status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", "Nieznany status. Dozwolone: backlog, watching, watched.")
		return
	}
	if len(req.Score) > 0 {
		p.ScoreSet = true
		if string(req.Score) != "null" {
			var sc int
			if err := json.Unmarshal(req.Score, &sc); err != nil || sc < 0 || sc > 100 {
				writeFieldError(w, http.StatusBadRequest, "invalid_score", "score", "Ocena to liczba całkowita od 0 do 100.")
				return
			}
			p.Score = &sc
		}
	}
	if req.Note != nil && len([]rune(*req.Note)) > maxNoteRunes {
		writeFieldError(w, http.StatusBadRequest, "note_too_long", "note", "Notatka może mieć najwyżej 5000 znaków.")
		return
	}
	ctx := r.Context()
	if req.WatchedEpisodes != nil {
		cur, err := s.media.MediaItem(ctx, userIDFrom(ctx), id)
		if err != nil {
			s.writeMediaErr(w, r, err)
			return
		}
		n := *req.WatchedEpisodes
		if n < 0 || (cur.Episodes != nil && n > *cur.Episodes) {
			msg := "Liczba obejrzanych odcinków nie może być ujemna."
			if cur.Episodes != nil {
				msg = "Liczba obejrzanych odcinków musi być od 0 do " + strconv.Itoa(*cur.Episodes) + "."
			}
			writeFieldError(w, http.StatusBadRequest, "invalid_episodes", "watchedEpisodes", msg)
			return
		}
	}
	m, err := s.media.UpdateMedia(ctx, userIDFrom(ctx), id, p)
	if err != nil {
		s.writeMediaErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mediaFromStore(m))
}

func (s *server) mediaEpisode(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaIDParam(r)
	if !ok {
		writeError(w, http.StatusNotFound, "media_not_found", msgMediaNotFound)
		return
	}
	m, err := s.media.IncrementEpisode(r.Context(), userIDFrom(r.Context()), id)
	if err != nil {
		s.writeMediaErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mediaFromStore(m))
}

func (s *server) deleteMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := mediaIDParam(r)
	if !ok {
		writeError(w, http.StatusNotFound, "media_not_found", msgMediaNotFound)
		return
	}
	if err := s.media.DeleteMedia(r.Context(), userIDFrom(r.Context()), id); err != nil {
		s.writeMediaErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
