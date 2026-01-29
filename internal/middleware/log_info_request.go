package logrequest

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// RequestLogger создает middleware для логирования HTTP-запросов
func RequestLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Засекаем время начала обработки
			start := time.Now()

			// Создаем ResponseWriter для отслеживания статуса и размера ответа
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Обрабатываем запрос
			next.ServeHTTP(ww, r)

			// Время обработки
			duration := time.Since(start)

			// Логируем информацию о запросе
			logger.LogAttrs(r.Context(), slog.LevelInfo, "HTTP request",
				slog.String("URI", r.RequestURI),
				slog.String("method", r.Method),
				slog.String("query", r.URL.RawQuery),
				slog.Int("status", ww.Status()),
				slog.Int("size", ww.BytesWritten()),
				slog.String("duration", duration.String()),
				slog.String("remote_ip", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
				slog.String("referer", r.Referer()),
				slog.String("protocol", r.Proto),
			)

			logger.LogAttrs(r.Context(), slog.LevelInfo, "HTTP response",
				slog.Int("status", ww.Status()),
				slog.Int("size", ww.BytesWritten()),
			)
		})
	}
}
