package translate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memCache struct {
	mu    sync.Mutex
	items map[string]struct {
		text string
		at   time.Time
	}
	now func() time.Time
}

func newMemCache(now func() time.Time) *memCache {
	return &memCache{items: map[string]struct {
		text string
		at   time.Time
	}{}, now: now}
}

func (m *memCache) Translation(_ context.Context, hash, lang string, newerThan time.Time) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[hash+lang]
	if !ok || !it.at.After(newerThan) {
		return "", false, nil
	}
	return it.text, true, nil
}

func (m *memCache) SaveTranslation(_ context.Context, hash, lang, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[hash+lang] = struct {
		text string
		at   time.Time
	}{text, m.now()}
	return nil
}

// fakeMyMemory „tłumaczy” przez dopisanie [pl] i liczy zapytania.
func fakeMyMemory(t *testing.T, reply func(q string) (int, any)) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("langpair") != "en|pl" {
			t.Errorf("langpair = %q", r.URL.Query().Get("langpair"))
		}
		q := r.URL.Query().Get("q")
		if len(q) > 500 {
			t.Errorf("kawałek ma %d bajtów", len(q))
		}
		code, body := reply(q)
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func ok(q string) (int, any) {
	return 200, map[string]any{"responseData": map[string]any{"translatedText": "[pl] " + q + " &amp; &#39;ok&#39;"}, "responseStatus": 200}
}

func TestToPolishUsesCache(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	srv, calls := fakeMyMemory(t, ok)
	tr := New(newMemCache(clock), WithEndpoint(srv.URL), WithClock(clock))
	ctx := context.Background()

	got, err := tr.ToPolish(ctx, "Rise, Tarnished.")
	if err != nil {
		t.Fatal(err)
	}
	if got != "[pl] Rise, Tarnished. & 'ok'" {
		t.Errorf("tłumaczenie = %q", got)
	}
	if _, err := tr.ToPolish(ctx, "  Rise, Tarnished.  "); err != nil || calls.Load() != 1 {
		t.Errorf("drugie wywołanie poszło do MyMemory (%d zapytań), err = %v", calls.Load(), err)
	}

	// Po 30 dniach wpis traktujemy jak brak.
	now = now.Add(31 * 24 * time.Hour)
	if _, err := tr.ToPolish(ctx, "Rise, Tarnished."); err != nil || calls.Load() != 2 {
		t.Errorf("stary wpis nie wygasł (%d zapytań), err = %v", calls.Load(), err)
	}
}

func TestToPolishKeepsParagraphsAndChunksLongText(t *testing.T) {
	srv, calls := fakeMyMemory(t, func(q string) (int, any) {
		return 200, map[string]any{"responseData": map[string]any{"translatedText": "PL"}, "responseStatus": "200"}
	})
	tr := New(newMemCache(time.Now), WithEndpoint(srv.URL))
	long := strings.Repeat("This is a sentence that is long enough. ", 30) // ok. 1200 bajtów
	got, err := tr.ToPolish(context.Background(), "First paragraph.\n\n"+long)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(got, "\n\n")
	if len(parts) != 2 || parts[0] != "PL" || !strings.HasPrefix(parts[1], "PL PL") {
		t.Errorf("wynik = %q", got)
	}
	if calls.Load() < 4 {
		t.Errorf("zapytań = %d, oczekiwano podziału długiego akapitu", calls.Load())
	}
}

func TestToPolishErrors(t *testing.T) {
	tests := []struct {
		name  string
		reply func(string) (int, any)
		want  error
	}{
		{"limit w treści", func(string) (int, any) {
			return 200, map[string]any{"responseData": map[string]any{"translatedText": "MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY"}, "responseStatus": 429}
		}, ErrQuota},
		{"quotaFinished", func(string) (int, any) {
			return 200, map[string]any{"responseData": map[string]any{"translatedText": "x"}, "responseStatus": 200, "quotaFinished": true}
		}, ErrQuota},
		{"HTTP 429", func(string) (int, any) { return 429, map[string]any{} }, ErrQuota},
		{"HTTP 500", func(string) (int, any) { return 500, map[string]any{} }, ErrUnavailable},
		{"responseStatus 403", func(string) (int, any) {
			return 200, map[string]any{"responseData": map[string]any{"translatedText": "INVALID LANGUAGE PAIR"}, "responseStatus": "403"}
		}, ErrUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := fakeMyMemory(t, tt.reply)
			cache := newMemCache(time.Now)
			tr := New(cache, WithEndpoint(srv.URL))
			if _, err := tr.ToPolish(context.Background(), "Hello."); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, oczekiwano %v", err, tt.want)
			}
			if len(cache.items) != 0 {
				t.Error("błąd trafił do cache")
			}
		})
	}
}

func TestChunks(t *testing.T) {
	tests := []struct {
		name string
		text string
		max  int
		want []string
	}{
		{"krótki", "One. Two.", 450, []string{"One. Two."}},
		{"po zdaniach", "One one. Two two. Three.", 10, []string{"One one.", "Two two.", "Three."}},
		{"długie zdanie po słowach", "aaaa bbbb cccc", 9, []string{"aaaa bbbb", "cccc"}},
		{"słowo dłuższe niż limit", "abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"nie tnie znaku UTF-8", "żżżżż", 3, []string{"ż", "ż", "ż", "ż", "ż"}},
		{"kropka w liczbie zostaje", "Version 2.5 is out. Yes.", 450, []string{"Version 2.5 is out. Yes."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Chunks(tt.text, tt.max)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("Chunks = %q, oczekiwano %q", got, tt.want)
			}
			for _, c := range got {
				if len(c) > tt.max {
					t.Errorf("kawałek %q ma %d bajtów", c, len(c))
				}
			}
		})
	}
}
