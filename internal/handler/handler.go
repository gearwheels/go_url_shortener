package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/service"
)

func ShortenHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "text/plain" {
		http.Error(w, "Unsupported Media Type. Expected text/plain",
			http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalURL := strings.TrimSpace(string(body))

	if originalURL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(originalURL, "http://") &&
		!strings.HasPrefix(originalURL, "https://") {
		originalURL = "http://" + originalURL
	}

	id := service.Shortener.ShortenURL(originalURL)

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Location", shortenedURL)
	w.WriteHeader(http.StatusCreated)

	fmt.Fprint(w, shortenedURL)

	log.Printf("Created short URL: %s for %s", shortenedURL, originalURL)
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {

	// Извлекаем ID из пути (убираем ведущий слэш)
	id := strings.TrimPrefix(r.URL.Path, "/")

	if id == "" {
		fmt.Fprint(w, "Send POST request with URL in body as text/plain to shorten URL")
		return
	}

	originalURL, exists := service.Shortener.GetOriginalURL(id)
	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	log.Printf("Redirecting %s -> %s", id, originalURL)
}
