// Package ranking liczy oceny gier z miejsc w rankingach kategorii.
// Same czyste funkcje: bez bazy, sieci i zegara.
//
// Ta sama logika jest w web/src/lib/ranking.ts (podgląd na żywo w oknie oceny).
// Oba testy czytają wspólne przypadki z testdata/cases.json.
package ranking

import (
	"math"
	"slices"
)

// Category to kategoria rankingu (enum ranking_category w bazie).
type Category string

const (
	Gameplay      Category = "gameplay"
	Story         Category = "story"
	Graphics      Category = "graphics"
	Audio         Category = "audio"
	Replayability Category = "replayability"
	Difficulty    Category = "difficulty"
)

// Categories w stałej kolejności (tej samej co w UI i w testach zgodności).
var Categories = []Category{Gameplay, Story, Graphics, Audio, Replayability, Difficulty}

// Weights to wagi kategorii w procentach (suma 100).
var Weights = map[Category]int{
	Gameplay:      35,
	Story:         25,
	Graphics:      15,
	Audio:         10,
	Replayability: 10,
	Difficulty:    5,
}

// Valid mówi, czy kategoria istnieje.
func (c Category) Valid() bool {
	_, ok := Weights[c]
	return ok
}

// Points to punkty za miejsce p (1 = najlepsze) w rankingu o n grach:
// 100 − (p−1)/(n−1)·40, a przy n == 1 zawsze 100. Pierwsze miejsce daje 100,
// ostatnie 60.
func Points(p, n int) float64 {
	if n <= 1 {
		return 100
	}
	return 100 - float64(p-1)/float64(n-1)*40
}

// Place to miejsce gry w jednej kategorii.
type Place struct {
	Pos int // 1-based
	N   int // liczba gier w rankingu tej kategorii
}

// Score to średnia ważona punktów z kategorii, w których gra ma miejsce,
// zaokrąglona do liczby całkowitej. Pominięte kategorie nie liczą się do wag.
// Bez żadnej kategorii zwraca nil.
func Score(places map[Category]Place) *int {
	var sum, weights float64
	for _, c := range Categories { // stała kolejność = ten sam wynik zmiennoprzecinkowy co w TS
		p, ok := places[c]
		if !ok {
			continue
		}
		w := float64(Weights[c])
		sum += Points(p.Pos, p.N) * w
		weights += w
	}
	if weights == 0 {
		return nil
	}
	s := int(math.Floor(sum/weights + 0.5)) // jak Math.round w JS (połówki w górę)
	return &s
}

// Rankings to kolejność gier (identyfikatory user_games) w każdej kategorii,
// od najlepszej.
type Rankings map[Category][]int64

// Places zwraca miejsca gry we wszystkich kategoriach, w których występuje.
func (r Rankings) Places(id int64) map[Category]Place {
	out := map[Category]Place{}
	for _, c := range Categories {
		list := r[c]
		if i := slices.Index(list, id); i >= 0 {
			out[c] = Place{Pos: i + 1, N: len(list)}
		}
	}
	return out
}

// Recompute liczy oceny wszystkich podanych gier (ukończonych gier użytkownika).
// Gry spoza rankingów dostają nil.
func Recompute(r Rankings, games []int64) map[int64]*int {
	out := make(map[int64]*int, len(games))
	for _, id := range games {
		out[id] = Score(r.Places(id))
	}
	return out
}

// Insert zwraca nową kolejność z grą id na pozycji pos (0-based). Jeśli gra
// już była w rankingu, najpierw ją usuwa. pos poza zakresem ląduje na końcu.
func Insert(list []int64, id int64, pos int) []int64 {
	out := Remove(list, id)
	pos = max(0, min(pos, len(out)))
	return slices.Insert(out, pos, id)
}

// Remove zwraca kolejność bez gry id.
func Remove(list []int64, id int64) []int64 {
	out := make([]int64, 0, len(list))
	for _, v := range list {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
