package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/handler"
	logRequest "github.com/gearwheels/go_url_shortener/internal/middleware"
)

func main() { // go run "d:\yandex_practice\go_url_shortener\cmd\shortener\main.go" -a localhost:8080 -b http://localhost:8080/
	router := chi.NewRouter()
	router.Use(middleware.RequestID)                 // Добавляет ID каждому запросу
	router.Use(middleware.RealIP)                    // Получает реальный IP
	router.Use(middleware.Recoverer)                 // Обработка паник
	router.Use(middleware.Timeout(60 * time.Second)) // Таймаут

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Кастомизация формата времени
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	}))

	slog.SetDefault(logger)

	a := flag.String("a", "localhost:8080", "start up address for the server")
	// пробросить в обработчики чтоб отдавать ответ с адресом b
	b := flag.String("b", "http://localhost:8080/", "destination folder")
	// разбор командной строки
	flag.Parse()
	config.Init(*a, *b)

	// Наш middleware для логирования
	router.Use(logRequest.RequestLogger(logger))
	router.Post("/", handler.ShortenHandler)
	router.Get("/{id}", handler.RedirectHandler)

	// port := ":8080"
	fmt.Printf("URL Shortener server starting on %s\n", *a)
	fmt.Println("\nEndpoints:")
	fmt.Println("  POST / - Shorten URL")
	fmt.Println("    Content-Type: text/plain")
	fmt.Println("    Body: URL to shorten")
	fmt.Println("    Response: 201 with shortened URL")
	fmt.Println()
	fmt.Println("  GET /{id} - Redirect to original URL")
	fmt.Println("    Response: 307 with Location header")

	if err := http.ListenAndServe(*a, router); err != nil {
		slog.Error("Server error:", slog.String("err", err.Error()))

	}
}
