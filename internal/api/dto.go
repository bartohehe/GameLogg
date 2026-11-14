package api

import (
	"time"

	"github.com/bartohehe/GameLogg/internal/igdb"
	"github.com/bartohehe/GameLogg/internal/store"
)

type spineDTO struct {
	Color  string `json:"color"`
	Accent string `json:"accent"`
	Ink    string `json:"ink"`
}

type gameDTO struct {
	IGDBID      int64     `json:"igdbId"`
	Title       string    `json:"title"`
	CoverURL    *string   `json:"coverUrl"`
	ReleaseDate *string   `json:"releaseDate"` // RRRR-MM-DD
	Year        *int      `json:"year"`
	Developer   string    `json:"developer"`
	Genres      []string  `json:"genres"`
	Platforms   []string  `json:"platforms"`
	IGDBRating  *int      `json:"igdbRating"`
	Spine       *spineDTO `json:"spine"` // null dla gier jeszcze niezapisanych w cache
}

type entryRefDTO struct {
	ID     int64        `json:"id"`
	Status store.Status `json:"status"`
}

type entryDTO struct {
	ID             int64        `json:"id"`
	Status         store.Status `json:"status"`
	Platform       string       `json:"platform"`
	Score          *int         `json:"score"`
	Note           string       `json:"note"`
	CompletedCount int          `json:"completedCount"`
	FinishedAt     *time.Time   `json:"finishedAt"`
	AddedAt        time.Time    `json:"addedAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	Game           gameDTO      `json:"game"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func dateFields(t *time.Time) (*string, *int) {
	if t == nil {
		return nil, nil
	}
	d := t.UTC().Format("2006-01-02")
	y := t.UTC().Year()
	return &d, &y
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func gameFromStore(g store.Game) gameDTO {
	d, y := dateFields(g.ReleaseDate)
	return gameDTO{
		IGDBID: g.IGDBID, Title: g.Title, CoverURL: strPtr(g.CoverURL), ReleaseDate: d, Year: y,
		Developer: g.Developer, Genres: nonNil(g.Genres), Platforms: nonNil(g.Platforms), IGDBRating: g.IGDBRating,
		Spine: &spineDTO{Color: g.SpineColor, Accent: g.SpineAccent, Ink: g.SpineInk},
	}
}

func gameFromIGDB(g igdb.Game) gameDTO {
	d, y := dateFields(g.ReleaseDate)
	return gameDTO{
		IGDBID: g.ID, Title: g.Title, CoverURL: strPtr(g.CoverURL), ReleaseDate: d, Year: y,
		Developer: g.Developer, Genres: nonNil(g.Genres), Platforms: nonNil(g.Platforms), IGDBRating: g.Rating,
	}
}

func entryFromStore(e store.LibraryEntry) entryDTO {
	return entryDTO{
		ID: e.ID, Status: e.Status, Platform: e.Platform, Score: e.Score, Note: e.Note,
		CompletedCount: e.CompletedCount, FinishedAt: e.FinishedAt, AddedAt: e.AddedAt, UpdatedAt: e.UpdatedAt,
		Game: gameFromStore(e.Game),
	}
}
