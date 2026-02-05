package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gearwheels/go_url_shortener/internal/config"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
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

	slog.Info("Created short URL: %s for %s", shortenedURL, originalURL)
}

func JSONShortenHandler(w http.ResponseWriter, r *http.Request) {

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Unsupported Media Type. Expected application/json",
			http.StatusUnsupportedMediaType)
		return
	}
	var buf bytes.Buffer
	var request schemasshortener.RequestSchema
	var response schemasshortener.ResponseSchema

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf.Bytes(), &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(request.URL, "http://") &&
		!strings.HasPrefix(request.URL, "https://") {
		request.URL = "http://" + request.URL
	}

	id := service.Shortener.ShortenURL(request.URL)

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)
	response.Result = shortenedURL
	resp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	w.Write(resp)

	slog.Info("Created short URL: %s for %s", shortenedURL, request.URL)
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {

	// Извлекаем ID из пути (убираем ведущий слэш)
	id := strings.TrimPrefix(r.URL.Path, "/")

	if id == "" {
		// fmt.Fprint(w, "Send POST request with URL in body as text/plain to shorten URL")
		http.Error(w, "Send POST request with URL in body as text/plain to shorten URL", http.StatusBadRequest)
		return
	}

	originalURL, exists := service.Shortener.GetOriginalURL(id)
	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	slog.Info("Redirecting %s -> %s", id, originalURL)
}
