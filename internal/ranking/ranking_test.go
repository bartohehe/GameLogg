package ranking

import (
	"encoding/json"
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
)

// Wspólne przypadki z web/src/lib/ranking.test.ts.
type cases struct {
	Points []struct {
		P, N int
		Want float64
	} `json:"points"`
	Scores []struct {
		Name   string
		Places map[Category]struct{ Pos, N int }
		Want   *int
	} `json:"scores"`
	Recompute []recomputeCase `json:"recompute"`
}

type recomputeCase struct {
	Name     string
	Rankings Rankings
	Games    []int64
	Want     map[string]*int
	Then     *struct {
		Insert *struct {
			Category Category
			ID       int64
			Pos      int
		}
		Remove   *int64
		Games    []int64
		Rankings Rankings
		Want     map[string]*int
	}
}

func loadCases(t *testing.T) cases {
	t.Helper()
	b, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var c cases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func fmtScore(s *int) string {
	if s == nil {
		return "nil"
	}
	return strconv.Itoa(*s)
}

func checkScores(t *testing.T, got map[int64]*int, want map[string]*int) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("ocen = %d, oczekiwano %d", len(got), len(want))
	}
	for k, w := range want {
		id, _ := strconv.ParseInt(k, 10, 64)
		if g := got[id]; fmtScore(g) != fmtScore(w) {
			t.Errorf("gra %d: ocena %s, oczekiwano %s", id, fmtScore(g), fmtScore(w))
		}
	}
}

func TestWeightsSumTo100(t *testing.T) {
	sum := 0
	for _, c := range Categories {
		sum += Weights[c]
	}
	if sum != 100 || len(Categories) != 6 {
		t.Errorf("suma wag = %d, kategorii %d", sum, len(Categories))
	}
}

func TestPoints(t *testing.T) {
	for _, tt := range loadCases(t).Points {
		if got := Points(tt.P, tt.N); math.Abs(got-tt.Want) > 1e-9 {
			t.Errorf("Points(%d, %d) = %v, oczekiwano %v", tt.P, tt.N, got, tt.Want)
		}
	}
}

func TestScore(t *testing.T) {
	for _, tt := range loadCases(t).Scores {
		t.Run(tt.Name, func(t *testing.T) {
			places := map[Category]Place{}
			for c, p := range tt.Places {
				places[c] = Place{Pos: p.Pos, N: p.N}
			}
			if got := Score(places); fmtScore(got) != fmtScore(tt.Want) {
				t.Errorf("Score = %s, oczekiwano %s", fmtScore(got), fmtScore(tt.Want))
			}
		})
	}
}

func TestRecompute(t *testing.T) {
	for _, tt := range loadCases(t).Recompute {
		t.Run(tt.Name, func(t *testing.T) {
			checkScores(t, Recompute(tt.Rankings, tt.Games), tt.Want)
			if tt.Then == nil {
				return
			}
			next := Rankings{}
			for c, list := range tt.Rankings {
				next[c] = slices.Clone(list)
			}
			switch {
			case tt.Then.Insert != nil:
				ins := tt.Then.Insert
				next[ins.Category] = Insert(next[ins.Category], ins.ID, ins.Pos)
			case tt.Then.Remove != nil:
				for c := range next {
					next[c] = Remove(next[c], *tt.Then.Remove)
				}
			}
			for _, c := range Categories {
				if !slices.Equal(next[c], tt.Then.Rankings[c]) {
					t.Errorf("%s: kolejność %v, oczekiwano %v", c, next[c], tt.Then.Rankings[c])
				}
			}
			checkScores(t, Recompute(next, tt.Then.Games), tt.Then.Want)
		})
	}
}

func TestInsertAndRemove(t *testing.T) {
	tests := []struct {
		name string
		list []int64
		id   int64
		pos  int
		want []int64
	}{
		{"do pustej", nil, 7, 0, []int64{7}},
		{"na początek", []int64{1, 2}, 3, 0, []int64{3, 1, 2}},
		{"w środek", []int64{1, 2, 3}, 4, 1, []int64{1, 4, 2, 3}},
		{"na koniec", []int64{1, 2}, 3, 2, []int64{1, 2, 3}},
		{"pozycja za końcem", []int64{1, 2}, 3, 99, []int64{1, 2, 3}},
		{"pozycja ujemna", []int64{1, 2}, 3, -5, []int64{3, 1, 2}},
		{"przeniesienie w dół", []int64{1, 2, 3}, 1, 2, []int64{2, 3, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := slices.Clone(tt.list)
			if got := Insert(tt.list, tt.id, tt.pos); !slices.Equal(got, tt.want) {
				t.Errorf("Insert = %v, oczekiwano %v", got, tt.want)
			}
			if !slices.Equal(tt.list, orig) {
				t.Error("Insert zmienił listę wejściową")
			}
		})
	}
	if got := Remove([]int64{1, 2, 3}, 2); !slices.Equal(got, []int64{1, 3}) {
		t.Errorf("Remove = %v", got)
	}
	if got := Remove([]int64{1}, 9); !slices.Equal(got, []int64{1}) {
		t.Errorf("Remove nieobecnej = %v", got)
	}
}

func TestPlaces(t *testing.T) {
	r := Rankings{Gameplay: {5, 6, 7}, Story: {7}}
	p := r.Places(7)
	if p[Gameplay] != (Place{3, 3}) || p[Story] != (Place{1, 1}) || len(p) != 2 {
		t.Errorf("Places = %+v", p)
	}
	if len(r.Places(99)) != 0 {
		t.Error("gra spoza rankingów ma miejsca")
	}
}
