package store

import (
	"context"
	"errors"
	"testing"
)

func intp(v int) *int { return &v }

func testMedia(uid int64, typ MediaType, ext string, episodes *int, status MediaStatus) Media {
	return Media{UserID: uid, Source: SourceTMDB, ExternalID: ext, Type: typ, Title: "Tytuł " + ext, Year: intp(2024),
		Creator: "Twórca", Episodes: episodes, Status: status,
		SpineColor: "#20305A", SpineAccent: "#F08A3E", SpineInk: "#FFFFFF"}
}

func migratedMediaStore(t *testing.T) *Store {
	t.Helper()
	s := migratedStore(t) // TRUNCATE users kaskadowo czyści też user_media
	return s
}

func TestAddAndListMedia(t *testing.T) {
	s := migratedMediaStore(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")

	film, err := s.AddMedia(ctx, testMedia(u, TypeFilm, "1", nil, MediaBacklog))
	if err != nil {
		t.Fatal(err)
	}
	if film.WatchedEpisodes != nil || film.Status != MediaBacklog || film.Genres == nil {
		t.Errorf("film = %+v", film)
	}
	done, _ := s.AddMedia(ctx, testMedia(u, TypeSerial, "2", intp(8), MediaWatched))
	if done.WatchedEpisodes == nil || *done.WatchedEpisodes != 8 {
		t.Errorf("obejrzany serial: odcinki %v", done.WatchedEpisodes)
	}
	watching, _ := s.AddMedia(ctx, testMedia(u, TypeSerial, "3", intp(10), MediaWatching))
	if watching.WatchedEpisodes == nil || *watching.WatchedEpisodes != 0 {
		t.Errorf("oglądany serial: odcinki %v", watching.WatchedEpisodes)
	}

	if _, err := s.AddMedia(ctx, testMedia(u, TypeFilm, "1", nil, MediaWatched)); !errors.Is(err, ErrAlreadyInLibrary) {
		t.Errorf("duplikat: err = %v", err)
	}

	all, _ := s.MediaList(ctx, u, "", "")
	series, _ := s.MediaList(ctx, u, TypeSerial, "")
	backlog, _ := s.MediaList(ctx, u, "", MediaBacklog)
	if len(all) != 3 || len(series) != 2 || len(backlog) != 1 {
		t.Errorf("listy: %d %d %d", len(all), len(series), len(backlog))
	}
	refs, _ := s.MediaRefs(ctx, u, SourceTMDB, []string{"1", "3", "99"})
	if len(refs) != 2 || refs["3"].Status != MediaWatching {
		t.Errorf("refs = %+v", refs)
	}
	if other, _ := s.MediaList(ctx, seedUser(t, s, "ewa"), "", ""); len(other) != 0 {
		t.Errorf("cudza lista widoczna: %d", len(other))
	}
}

func TestEpisodesAndStatus(t *testing.T) {
	s := migratedMediaStore(t)
	ctx := context.Background()
	u := seedUser(t, s, "jan")
	m, _ := s.AddMedia(ctx, testMedia(u, TypeAnime, "5", intp(3), MediaBacklog))

	// +1 z „do obejrzenia” przechodzi do „oglądam”.
	m, err := s.IncrementEpisode(ctx, u, m.ID)
	if err != nil || *m.WatchedEpisodes != 1 || m.Status != MediaWatching {
		t.Fatalf("pierwszy odcinek: %+v, %v", m, err)
	}
	m, _ = s.IncrementEpisode(ctx, u, m.ID)
	m, _ = s.IncrementEpisode(ctx, u, m.ID)
	if *m.WatchedEpisodes != 3 || m.Status != MediaWatched {
		t.Errorf("ostatni odcinek: %d %s", *m.WatchedEpisodes, m.Status)
	}
	if _, err := s.IncrementEpisode(ctx, u, m.ID); !errors.Is(err, ErrAllWatched) {
		t.Errorf("ponad liczbę odcinków: err = %v", err)
	}

	// Ręczna zmiana licznika, oceny i notatki; potem czyszczenie oceny.
	note := "Świetne."
	m, err = s.UpdateMedia(ctx, u, m.ID, MediaPatch{WatchedEpisodes: intp(2), ScoreSet: true, Score: intp(93), Note: &note,
		Status: ptr(MediaWatching)})
	if err != nil || *m.WatchedEpisodes != 2 || *m.Score != 93 || m.Note != note || m.Status != MediaWatching {
		t.Errorf("patch: %+v, %v", m, err)
	}
	m, _ = s.UpdateMedia(ctx, u, m.ID, MediaPatch{ScoreSet: true})
	if m.Score != nil {
		t.Errorf("ocena nie została wyczyszczona: %v", *m.Score)
	}
	// Obejrzane uzupełnia licznik.
	m, _ = s.UpdateMedia(ctx, u, m.ID, MediaPatch{Status: ptr(MediaWatched)})
	if *m.WatchedEpisodes != 3 {
		t.Errorf("obejrzane: odcinki %d", *m.WatchedEpisodes)
	}

	// Ongoing anime bez znanej liczby odcinków: licznik rośnie bez końca.
	ongoing, _ := s.AddMedia(ctx, testMedia(u, TypeAnime, "6", nil, MediaWatching))
	for range 2 {
		ongoing, _ = s.IncrementEpisode(ctx, u, ongoing.ID)
	}
	if *ongoing.WatchedEpisodes != 2 || ongoing.Status != MediaWatching {
		t.Errorf("ongoing: %+v", ongoing)
	}

	film, _ := s.AddMedia(ctx, testMedia(u, TypeFilm, "7", nil, MediaBacklog))
	if _, err := s.IncrementEpisode(ctx, u, film.ID); !errors.Is(err, ErrNotEpisodic) {
		t.Errorf("film +1: err = %v", err)
	}
	if _, err := s.UpdateMedia(ctx, u, film.ID, MediaPatch{WatchedEpisodes: intp(1)}); !errors.Is(err, ErrNotEpisodic) {
		t.Errorf("film licznik: err = %v", err)
	}

	intruder := seedUser(t, s, "ewa")
	if _, err := s.IncrementEpisode(ctx, intruder, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cudzy +1: err = %v", err)
	}
	if err := s.DeleteMedia(ctx, intruder, m.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cudze usuwanie: err = %v", err)
	}
	if err := s.DeleteMedia(ctx, u, m.ID); err != nil {
		t.Errorf("usuwanie: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }
