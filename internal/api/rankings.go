package api

import (
	"errors"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/bartohehe/gamelogg/internal/ranking"
	"github.com/bartohehe/gamelogg/internal/store"
)

const maxRankingSize = 5000

func categoryParam(w http.ResponseWriter, r *http.Request) (ranking.Category, bool) {
	c := ranking.Category(chi.URLParam(r, "category"))
	if !c.Valid() {
		writeError(w, http.StatusNotFound, "unknown_category",
			"Nie ma takiej kategorii. Dozwolone: gameplay, story, graphics, audio, replayability, difficulty.")
		return "", false
	}
	return c, true
}

type rankedDTO struct {
	Position int      `json:"position"`
	Entry    entryDTO `json:"entry"`
}

func (s *server) getRanking(w http.ResponseWriter, r *http.Request) {
	cat, ok := categoryParam(w, r)
	if !ok {
		return
	}
	list, err := s.lib.Ranking(r.Context(), userIDFrom(r.Context()), cat)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	items := make([]rankedDTO, len(list))
	for i, re := range list {
		items[i] = rankedDTO{Position: re.Position, Entry: entryFromStore(re.Entry)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"category": cat, "items": items})
}

type putRankingRequest struct {
	UserGameIDs *[]int64 `json:"userGameIds"`
}

type scoreDTO struct {
	ID    int64 `json:"id"`
	Score *int  `json:"score"`
}

func (s *server) putRanking(w http.ResponseWriter, r *http.Request) {
	cat, ok := categoryParam(w, r)
	if !ok {
		return
	}
	var req putRankingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.UserGameIDs == nil {
		writeFieldError(w, http.StatusBadRequest, "missing_ids", "userGameIds",
			"Podaj pełną kolejność gier w polu userGameIds (pusta lista czyści ranking).")
		return
	}
	if len(*req.UserGameIDs) > maxRankingSize {
		writeFieldError(w, http.StatusBadRequest, "ranking_too_long", "userGameIds", "Za długa lista gier.")
		return
	}

	scores, err := s.lib.SetRanking(r.Context(), userIDFrom(r.Context()), cat, *req.UserGameIDs)
	if errors.Is(err, store.ErrInvalidRanking) {
		writeFieldError(w, http.StatusBadRequest, "invalid_ranking", "userGameIds",
			"Ranking może zawierać tylko ukończone gry z twojej biblioteki, każdą raz.")
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	out := make([]scoreDTO, 0, len(scores))
	for id, sc := range scores {
		out = append(out, scoreDTO{ID: id, Score: sc})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"scores": out})
}

type categoryPlaceDTO struct {
	Category ranking.Category `json:"category"`
	Weight   int              `json:"weight"`
	Position *int             `json:"position"` // null = gra pominięta w tej kategorii
	N        int              `json:"n"`        // liczba gier w rankingu kategorii
	Points   *float64         `json:"points"`
}

func (s *server) entryRankings(w http.ResponseWriter, r *http.Request) {
	id, ok := entryIDParam(r)
	if !ok {
		writeEntryNotFound(w)
		return
	}
	ctx := r.Context()
	places, sizes, err := s.lib.EntryPlaces(ctx, userIDFrom(ctx), id)
	if errors.Is(err, store.ErrNotFound) {
		writeEntryNotFound(w)
		return
	}
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	cats := make([]categoryPlaceDTO, len(ranking.Categories))
	for i, c := range ranking.Categories {
		d := categoryPlaceDTO{Category: c, Weight: ranking.Weights[c], N: sizes[c]}
		if p, ok := places[c]; ok {
			pos, pts := p.Pos, ranking.Points(p.Pos, p.N)
			d.Position, d.Points = &pos, &pts
		}
		cats[i] = d
	}
	writeJSON(w, http.StatusOK, map[string]any{"score": ranking.Score(places), "categories": cats})
}
