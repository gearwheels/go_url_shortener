package handler

import (
	"bytes"
	"database/sql"
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

	id, inserted, err := service.Shortener.ShortenURL(r.Context(), originalURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL", "error", err)
		return
	}

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Location", shortenedURL)
	if inserted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusConflict)
	}

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

	id, inserted, err := service.Shortener.ShortenURL(r.Context(), request.URL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL", "error", err)
		return
	}

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)
	response.Result = shortenedURL
	resp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to marshal JSON", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if inserted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusConflict)
	}

	w.Write(resp)

	slog.Info("Created short URL: %s for %s", shortenedURL, request.URL)
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из пути (убираем ведущий слэш)
	id := strings.TrimPrefix(r.URL.Path, "/")

	if id == "" {
		http.Error(w, "Send POST request with URL in body as text/plain to shorten URL", http.StatusBadRequest)
		return
	}

	originalURL, err := service.Shortener.GetOriginalURL(r.Context(), id)
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	slog.Info("Redirecting %s -> %s", id, originalURL)
}

func CheckDBStatus(w http.ResponseWriter, r *http.Request) {

	db, err := sql.Open("pgx", config.AppConfig.DatabaseDsn)
	if err != nil {
		slog.Error("Ошибка открытия соединения: " + err.Error())
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusInternalServerError)
		return
	} else {
		w.WriteHeader(http.StatusOK)
		slog.Info("Data base alive!")
		return
	}
}

func ShortenBatchHandler(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Unsupported Media Type. Expected application/json",
			http.StatusUnsupportedMediaType)
		return
	}
	var buf bytes.Buffer
	var batchURL []schemasshortener.RequestBatchURLSchema

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf.Bytes(), &batchURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(batchURL) == 0 {
		http.Error(w, "Batch of URL cannot be empty", http.StatusBadRequest)
		return
	}
	
	response, err := service.Shortener.ShortenURLBatch(r.Context(), batchURL)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL batch", "error", err)
		return
	}

	resp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to marshal JSON", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	w.Write(resp)
}
