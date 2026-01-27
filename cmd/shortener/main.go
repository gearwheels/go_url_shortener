package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"github.com/go-chi/chi/v5"

	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/handler"
)


func main() {// go run "d:\yandex_practice\go_url_shortener\cmd\shortener\main.go" -a localhost:8080 -b http://localhost:8080/
	router := chi.NewRouter()

	a := flag.String("a", "localhost:8080", "start up address for the server")
	// пробросить в обработчики чтоб отдавать ответ с адресом b
	b := flag.String("b", "http://localhost:8080/", "destination folder")
	// разбор командной строки
	flag.Parse()
	config.Init(*a, *b)


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
		log.Fatal("Server error:", err)
	}
}
