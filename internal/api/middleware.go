package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// requestLogger loguje każde żądanie przez slog: metoda, ścieżka, status, czas.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}
				level := slog.LevelDebug
				switch {
				case status >= 500:
					level = slog.LevelError
				case status >= 400:
					level = slog.LevelInfo
				}
				log.Log(r.Context(), level, "http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", status,
					"bytes", ww.BytesWritten(),
					"duration", time.Since(start),
					"request_id", middleware.GetReqID(r.Context()),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}
