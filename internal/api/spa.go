package api

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// spaHandler serwuje pliki zbudowanego frontendu, a dla nieznanych ścieżek
// zwraca index.html, żeby routing po stronie przeglądarki działał po odświeżeniu.
type spaHandler struct {
	files     fs.FS
	index     []byte
	hasIndex  bool
	fileServe http.Handler
}

const noFrontendPage = `<!doctype html>
<html lang="pl"><head><meta charset="utf-8"><title>GameLogg</title></head>
<body style="font-family:system-ui,sans-serif;background:#1B1923;color:#EEEAF4;padding:40px">
<h1>Frontend nie jest wbudowany</h1>
<p>Ta binarka powstała bez zbudowanego frontendu. Uruchom <code>npm run build</code> w katalogu <code>web</code>
i zbuduj aplikację ponownie (<code>make build</code>), albo użyj obrazu Dockera.</p>
</body></html>`

func newSPAHandler(files fs.FS) *spaHandler {
	h := &spaHandler{files: files}
	if files != nil {
		if b, err := fs.ReadFile(files, "index.html"); err == nil {
			h.index = b
			h.hasIndex = true
		}
		h.fileServe = http.FileServerFS(files)
	}
	return h
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.hasIndex {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(noFrontendPage))
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name != "" && name != "index.html" {
		if st, err := fs.Stat(h.files, name); err == nil && !st.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				// Vite dokleja hash do nazw w assets/, więc można je cache'ować na zawsze.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			h.fileServe.ServeHTTP(w, r)
			return
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Brakujący plik z rozszerzeniem (np. /assets/stary.js) to 404, a nie index.html.
		if path.Ext(name) != "" {
			http.NotFound(w, r)
			return
		}
	}

	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(h.index))
}
