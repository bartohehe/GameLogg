package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bartohehe/gamelogg/internal/ranking"
	"github.com/bartohehe/gamelogg/internal/store"
)

// LibraryStore to operacje na grach i bibliotece używane przez API.
type LibraryStore interface {
	GameByIGDBID(ctx context.Context, igdbID int64) (store.Game, error)
	UpsertGame(ctx context.Context, g store.Game) (store.Game, error)
	Library(ctx context.Context, userID int64, status store.Status, sort string) ([]store.LibraryEntry, error)
	Entry(ctx context.Context, userID, id int64) (store.LibraryEntry, error)
	EntryByIGDBID(ctx context.Context, userID, igdbID int64) (store.LibraryEntry, error)
	EntriesByIGDBIDs(ctx context.Context, userID int64, igdbIDs []int64) (map[int64]store.EntryRef, error)
	AddToLibrary(ctx context.Context, userID, gameID int64, status store.Status, platform string) (store.LibraryEntry, error)
	UpdateEntry(ctx context.Context, userID, id int64, p store.EntryPatch) (store.LibraryEntry, error)
	DeleteEntry(ctx context.Context, userID, id int64) error
	Ranking(ctx context.Context, userID int64, cat ranking.Category) ([]store.RankedEntry, error)
	SetRanking(ctx context.Context, userID int64, cat ranking.Category, ids []int64) (map[int64]*int, error)
	EntryPlaces(ctx context.Context, userID, entryID int64) (map[ranking.Category]ranking.Place, map[ranking.Category]int, error)
	TopRated(ctx context.Context, userID int64, limit int) ([]store.LibraryEntry, error)
	Stats(ctx context.Context, userID int64, year int) (store.Stats, error)
}

const maxNoteRunes = 5000

const (
	msgInvalidStatus = "Nieznany status. Dozwolone: planned, in_progress, completed, on_hold, wishlist, abandoned."
	msgEntryNotFound = "Tej gry nie ma w twojej bibliotece."
)

func writeEntryNotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "entry_not_found", msgEntryNotFound)
}

func entryIDParam(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id, err == nil && id > 0
}

// validPlatform: przy grze z listą platform wybór musi z niej pochodzić,
// a przy grze bez platform w IGDB zostaje pusty.
func validPlatform(platform string, gamePlatforms []string) bool {
	if len(gamePlatforms) == 0 {
		return platform == ""
	}
	return slices.Contains(gamePlatforms, platform)
}

func (s *server) listLibrary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := store.Status(q.Get("status"))
	if status != "" && !status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", msgInvalidStatus)
		return
	}
	sort := q.Get("sort")
	if sort == "" {
		sort = "score"
	}
	if !store.ValidSort(sort) {
		writeFieldError(w, http.StatusBadRequest, "invalid_sort", "sort", "Nieznane sortowanie. Dozwolone: score, title, added.")
		return
	}
	entries, err := s.lib.Library(r.Context(), userIDFrom(r.Context()), status, sort)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	items := make([]entryDTO, len(entries))
	for i, e := range entries {
		items[i] = entryFromStore(e)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type addRequest struct {
	IGDBID   int64        `json:"igdbId"`
	Status   store.Status `json:"status"`
	Platform string       `json:"platform"`
}

func (s *server) addToLibrary(w http.ResponseWriter, r *http.Request) {
	var req addRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IGDBID <= 0 {
		writeFieldError(w, http.StatusBadRequest, "invalid_igdb_id", "igdbId", "Brak identyfikatora gry z IGDB.")
		return
	}
	if !req.Status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", msgInvalidStatus)
		return
	}
	ctx := r.Context()
	game, err := s.ensureGame(ctx, req.IGDBID)
	switch {
	case errors.Is(err, errIGDBNotConfigured):
		s.igdbNotConfigured(w)
		return
	case err != nil:
		s.writeIGDBError(w, r, err)
		return
	}

	req.Platform = strings.TrimSpace(req.Platform)
	if !validPlatform(req.Platform, game.Platforms) {
		writeFieldError(w, http.StatusBadRequest, "invalid_platform", "platform",
			"Wybierz jedną z platform, na które gra wyszła.")
		return
	}

	e, err := s.lib.AddToLibrary(ctx, userIDFrom(ctx), game.ID, req.Status, req.Platform)
	if errors.Is(err, store.ErrAlreadyInLibrary) {
		writeError(w, http.StatusConflict, "already_in_library", "Ta gra już stoi na twojej półce.")
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, entryFromStore(e))
}

type patchRequest struct {
	Status   *store.Status `json:"status"`
	Platform *string       `json:"platform"`
	Note     *string       `json:"note"`
}

func (s *server) patchEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := entryIDParam(r)
	if !ok {
		writeEntryNotFound(w)
		return
	}
	var req patchRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	uid := userIDFrom(ctx)

	if req.Status != nil && !req.Status.Valid() {
		writeFieldError(w, http.StatusBadRequest, "invalid_status", "status", msgInvalidStatus)
		return
	}
	if req.Note != nil && len([]rune(*req.Note)) > maxNoteRunes {
		writeFieldError(w, http.StatusBadRequest, "note_too_long", "note", "Notatka może mieć najwyżej 5000 znaków.")
		return
	}
	if req.Platform != nil {
		*req.Platform = strings.TrimSpace(*req.Platform)
		cur, err := s.lib.Entry(ctx, uid, id)
		if errors.Is(err, store.ErrNotFound) {
			writeEntryNotFound(w)
			return
		}
		if err != nil {
			writeInternal(w, r, s.log, err)
			return
		}
		if !validPlatform(*req.Platform, cur.Game.Platforms) {
			writeFieldError(w, http.StatusBadRequest, "invalid_platform", "platform",
				"Wybierz jedną z platform, na które gra wyszła.")
			return
		}
	}

	e, err := s.lib.UpdateEntry(ctx, uid, id, store.EntryPatch{Status: req.Status, Platform: req.Platform, Note: req.Note})
	if errors.Is(err, store.ErrNotFound) {
		writeEntryNotFound(w)
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, entryFromStore(e))
}

func (s *server) deleteEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := entryIDParam(r)
	if !ok {
		writeEntryNotFound(w)
		return
	}
	err := s.lib.DeleteEntry(r.Context(), userIDFrom(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		writeEntryNotFound(w)
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
