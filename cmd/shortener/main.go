package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

type URLShortener struct {
	mu    sync.RWMutex
	store map[string]string // короткий ID -> оригинальный URL
}

func NewURLShortener() *URLShortener {
	return &URLShortener{
		store: make(map[string]string),
	}
}

func (us *URLShortener) generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (us *URLShortener) shortenURL(originalURL string) string {
	us.mu.Lock()
	defer us.mu.Unlock()

	// Проверяем, есть ли уже такой URL в хранилище
	for id, url := range us.store {
		if url == originalURL {
			return id // Возвращаем существующий ID
		}
	}
	// Генерируем уникальный ID
	var id string
	for {
		id = us.generateID()
		if _, exists := us.store[id]; !exists {
			break
		}
	}
	// Сохраняем в хранилище
	us.store[id] = originalURL
	log.Printf("Shortened URL: %s -> %s", id, originalURL)
	return id
}

func (us *URLShortener) getOriginalURL(id string) (string, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	url, exists := us.store[id]
	return url, exists
}

func (us *URLShortener) shortenHandler(w http.ResponseWriter, r *http.Request) {

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

	id := us.shortenURL(originalURL)

	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	shortenedURL := fmt.Sprintf("http://%s/%s", host, id)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Location", shortenedURL)
	w.WriteHeader(http.StatusCreated)

	fmt.Fprint(w, shortenedURL)

	log.Printf("Created short URL: %s for %s", shortenedURL, originalURL)
}

func (us *URLShortener) redirectHandler(w http.ResponseWriter, r *http.Request) {

	// Извлекаем ID из пути (убираем ведущий слэш)
	id := strings.TrimPrefix(r.URL.Path, "/")

	if id == "" {
		fmt.Fprint(w, "Send POST request with URL in body as text/plain to shorten URL")
		return
	}

	originalURL, exists := us.getOriginalURL(id)
	if !exists {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)

	log.Printf("Redirecting %s -> %s", id, originalURL)
}

func (us *URLShortener) mainHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		us.shortenHandler(w, r)
	case http.MethodGet:
		us.redirectHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	shortener := NewURLShortener()

	http.HandleFunc("/", shortener.mainHandler)

	port := ":8080"
	fmt.Printf("URL Shortener server starting on http://localhost%s\n", port)
	fmt.Println("\nEndpoints:")
	fmt.Println("  POST / - Shorten URL")
	fmt.Println("    Content-Type: text/plain")
	fmt.Println("    Body: URL to shorten")
	fmt.Println("    Response: 201 with shortened URL")
	fmt.Println()
	fmt.Println("  GET /{id} - Redirect to original URL")
	fmt.Println("    Response: 307 with Location header")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server error:", err)
	}
}
