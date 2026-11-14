package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/bartohehe/gamelogg/internal/igdb"
	"github.com/bartohehe/gamelogg/internal/store"
)

// Translator tłumaczy opisy EN→PL (nil, gdy TRANSLATION_ENABLED=false).
type Translator interface {
	ToPolish(ctx context.Context, text string) (string, error)
}

const (
	// Gry, z których bierzemy podobne tytuły: 3–5 najwyżej ocenionych.
	recommendationSources = 5
	maxRecommendations    = 12
	translateTimeout      = 6 * time.Second
)

func (s *server) popularGames(w http.ResponseWriter, r *http.Request) {
	if s.games == nil {
		s.igdbNotConfigured(w)
		return
	}
	found, err := s.games.Popular(r.Context())
	if err != nil {
		s.writeIGDBError(w, r, err)
		return
	}
	out, err := s.withEntries(r.Context(), found)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// withEntries dokleja do gier z IGDB informację „Na półce: …”.
func (s *server) withEntries(ctx context.Context, found []igdb.Game) ([]searchResultDTO, error) {
	ids := make([]int64, len(found))
	for i, g := range found {
		ids[i] = g.ID
	}
	owned, err := s.lib.EntriesByIGDBIDs(ctx, userIDFrom(ctx), ids)
	if err != nil {
		return nil, err
	}
	out := make([]searchResultDTO, len(found))
	for i, g := range found {
		out[i] = searchResultDTO{gameDTO: gameFromIGDB(g)}
		if ref, ok := owned[g.ID]; ok {
			out[i].Entry = &entryRefDTO{ID: ref.ID, Status: ref.Status}
		}
	}
	return out, nil
}

type becauseDTO struct {
	IGDBID int64  `json:"igdbId"`
	Title  string `json:"title"`
	Score  int    `json:"score"`
}

type recommendationDTO struct {
	Game    gameDTO    `json:"game"`
	Because becauseDTO `json:"because"`
}

func (s *server) recommendations(w http.ResponseWriter, r *http.Request) {
	if s.games == nil {
		s.igdbNotConfigured(w)
		return
	}
	ctx := r.Context()
	uid := userIDFrom(ctx)

	sources, err := s.lib.TopRated(ctx, uid, recommendationSources)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	if len(sources) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"items": []recommendationDTO{}, "basedOn": 0})
		return
	}
	ids := make([]int64, len(sources))
	for i, e := range sources {
		ids[i] = e.Game.IGDBID
	}
	similar, err := s.games.Similar(ctx, ids)
	if err != nil {
		s.writeIGDBError(w, r, err)
		return
	}

	var candidates []int64
	for _, list := range similar {
		for _, g := range list {
			candidates = append(candidates, g.ID)
		}
	}
	owned, err := s.lib.EntriesByIGDBIDs(ctx, uid, candidates)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}

	// Po kolei z każdej gry źródłowej (od najwyżej ocenionej), żeby polecenia
	// nie pochodziły wszystkie od jednej gry. Bez gier z biblioteki i duplikatów.
	items := []recommendationDTO{}
	seen := map[int64]bool{}
	next := make([]int, len(sources))
	for added := true; added && len(items) < maxRecommendations; {
		added = false
		for i, src := range sources {
			list := similar[src.Game.IGDBID]
			for next[i] < len(list) {
				g := list[next[i]]
				next[i]++
				if seen[g.ID] || g.ID == src.Game.IGDBID {
					continue
				}
				if _, have := owned[g.ID]; have {
					continue
				}
				seen[g.ID] = true
				items = append(items, recommendationDTO{
					Game:    gameFromIGDB(g),
					Because: becauseDTO{IGDBID: src.Game.IGDBID, Title: src.Game.Title, Score: *src.Score},
				})
				added = true
				break
			}
			if len(items) >= maxRecommendations {
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "basedOn": len(sources)})
}

type yearStatsDTO struct {
	Year         int        `json:"year"`
	Completed    int        `json:"completed"`
	Abandoned    int        `json:"abandoned"`
	AverageScore *int       `json:"averageScore"`
	Top          []entryDTO `json:"top"`
}

type statsDTO struct {
	Counts       map[store.Status]int `json:"counts"`
	Total        int                  `json:"total"`
	AverageScore *int                 `json:"averageScore"`
	Top          []entryDTO           `json:"top"`
	Year         yearStatsDTO         `json:"year"`
}

func entriesDTO(es []store.LibraryEntry) []entryDTO {
	out := make([]entryDTO, len(es))
	for i, e := range es {
		out[i] = entryFromStore(e)
	}
	return out
}

func (s *server) stats(w http.ResponseWriter, r *http.Request) {
	// Rok podaje klient (jego strefa czasowa); domyślnie bieżący rok UTC.
	year := time.Now().UTC().Year()
	if raw := r.URL.Query().Get("year"); raw != "" {
		y, err := strconv.Atoi(raw)
		if err != nil || y < 1970 || y > 3000 {
			writeFieldError(w, http.StatusBadRequest, "invalid_year", "year", "Rok musi być liczbą od 1970 do 3000.")
			return
		}
		year = y
	}
	st, err := s.lib.Stats(r.Context(), userIDFrom(r.Context()), year)
	if err != nil {
		writeInternal(w, r, s.log, err)
		return
	}
	writeJSON(w, http.StatusOK, statsDTO{
		Counts: st.Counts, Total: st.Total, AverageScore: st.AverageScore, Top: entriesDTO(st.Top),
		Year: yearStatsDTO{
			Year: st.Year.Year, Completed: st.Year.Completed, Abandoned: st.Year.Abandoned,
			AverageScore: st.Year.AverageScore, Top: entriesDTO(st.Year.Top),
		},
	})
}

// translateSummary zwraca polskie tłumaczenie opisu albo nil, gdy tłumaczenie
// jest wyłączone albo się nie udało (UI pokaże wtedy oryginał).
func (s *server) translateSummary(ctx context.Context, summary string) *string {
	if s.translator == nil || summary == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, translateTimeout)
	defer cancel()
	pl, err := s.translator.ToPolish(ctx, summary)
	if err != nil {
		s.log.WarnContext(ctx, "translation failed, showing original", "err", err)
		if pl == "" {
			return nil
		}
	}
	return &pl
}
