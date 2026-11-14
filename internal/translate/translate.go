// Package translate tłumaczy opisy gier EN→PL przez MyMemory, z cache
// w Postgresie (tabela translation_cache).
package translate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const (
	defaultEndpoint = "https://api.mymemory.translated.net/get"
	// CacheTTL: starsze tłumaczenia traktujemy jak brak.
	CacheTTL = 30 * 24 * time.Hour
	// MyMemory przyjmuje do 500 bajtów naraz; zostawiamy zapas.
	maxChunkBytes = 450
	targetLang    = "pl"
)

var (
	// ErrQuota: wyczerpany dzienny limit darmowych tłumaczeń MyMemory.
	ErrQuota = errors.New("mymemory: wyczerpany dzienny limit tłumaczeń")
	// ErrUnavailable: MyMemory nie odpowiada albo zwraca błąd.
	ErrUnavailable = errors.New("mymemory: usługa niedostępna")
)

// Cache przechowuje tłumaczenia (implementuje go store.Store).
type Cache interface {
	Translation(ctx context.Context, hash, lang string, newerThan time.Time) (string, bool, error)
	SaveTranslation(ctx context.Context, hash, lang, text string) error
}

// Translator tłumaczy tekst angielski na polski.
type Translator struct {
	cache    Cache
	http     *http.Client
	endpoint string
	now      func() time.Time
}

// Option zmienia ustawienia (używane w testach).
type Option func(*Translator)

// WithEndpoint podmienia adres MyMemory.
func WithEndpoint(u string) Option { return func(t *Translator) { t.endpoint = u } }

// WithClock podmienia zegar.
func WithClock(now func() time.Time) Option { return func(t *Translator) { t.now = now } }

// New tworzy tłumacza z cache.
func New(cache Cache, opts ...Option) *Translator {
	t := &Translator{
		cache:    cache,
		http:     &http.Client{Timeout: 8 * time.Second},
		endpoint: defaultEndpoint,
		now:      time.Now,
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

// Hash to klucz cache: SHA-256 tekstu źródłowego.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// ToPolish tłumaczy tekst. Najpierw szuka w cache (wpisy młodsze niż 30 dni),
// potem pyta MyMemory i zapisuje wynik. Akapity tłumaczy osobno, żeby zachować
// podział, a długie akapity kawałkami.
func (t *Translator) ToPolish(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	hash := Hash(text)
	if cached, ok, err := t.cache.Translation(ctx, hash, targetLang, t.now().Add(-CacheTTL)); err == nil && ok {
		return cached, nil
	}

	var paragraphs []string
	for _, para := range strings.Split(text, "\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		var parts []string
		for _, chunk := range Chunks(para, maxChunkBytes) {
			tr, err := t.translateChunk(ctx, chunk)
			if err != nil {
				return "", err
			}
			parts = append(parts, tr)
		}
		paragraphs = append(paragraphs, strings.Join(parts, " "))
	}
	result := strings.Join(paragraphs, "\n\n")
	if err := t.cache.SaveTranslation(ctx, hash, targetLang, result); err != nil {
		return result, fmt.Errorf("zapis tłumaczenia w cache: %w", err)
	}
	return result, nil
}

type myMemoryResponse struct {
	ResponseData struct {
		TranslatedText string `json:"translatedText"`
	} `json:"responseData"`
	ResponseStatus json.RawMessage `json:"responseStatus"` // raz liczba, raz napis
	QuotaFinished  bool            `json:"quotaFinished"`
}

func (t *Translator) translateChunk(ctx context.Context, chunk string) (string, error) {
	q := url.Values{"q": {chunk}, "langpair": {"en|pl"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if resp.StatusCode == http.StatusTooManyRequests {
		return "", ErrQuota
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	var r myMemoryResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return "", fmt.Errorf("%w: niepoprawna odpowiedź", ErrUnavailable)
	}
	status := strings.Trim(string(r.ResponseStatus), `"`)
	text := r.ResponseData.TranslatedText
	switch {
	case r.QuotaFinished, status == "429", strings.HasPrefix(text, "MYMEMORY WARNING"):
		return "", ErrQuota
	case status != "200":
		return "", fmt.Errorf("%w: responseStatus %s: %s", ErrUnavailable, status, text)
	case strings.TrimSpace(text) == "":
		return "", fmt.Errorf("%w: puste tłumaczenie", ErrUnavailable)
	}
	return html.UnescapeString(text), nil
}

// Chunks dzieli akapit na kawałki najwyżej maxBytes bajtów: po zdaniach,
// a gdy zdanie jest za długie, po słowach (i w ostateczności w środku słowa,
// na granicy znaku UTF-8).
func Chunks(text string, maxBytes int) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	add := func(piece string) {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			return
		}
		if cur.Len() > 0 && cur.Len()+1+len(piece) > maxBytes {
			flush()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(piece)
	}

	for _, sentence := range sentences(text) {
		if len(strings.TrimSpace(sentence)) <= maxBytes {
			add(sentence)
			continue
		}
		for _, w := range strings.Fields(sentence) {
			for len(w) > maxBytes {
				cut := maxBytes
				for cut > 0 && !utf8Start(w[cut]) {
					cut--
				}
				add(w[:cut])
				w = w[cut:]
			}
			add(w)
		}
	}
	flush()
	return out
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

// sentences dzieli tekst po kropce, wykrzykniku i pytajniku, po których
// jest spacja albo koniec tekstu.
func sentences(p string) []string {
	var out []string
	runes := []rune(p)
	start := 0
	for i, r := range runes {
		if (r == '.' || r == '!' || r == '?') && (i+1 == len(runes) || unicode.IsSpace(runes[i+1])) {
			out = append(out, string(runes[start:i+1]))
			start = i + 1
		}
	}
	if start < len(runes) {
		out = append(out, string(runes[start:]))
	}
	return out
}
