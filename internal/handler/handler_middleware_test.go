package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/gearwheels/go_url_shortener/internal/config"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	"github.com/gearwheels/go_url_shortener/internal/service"
	schemasShortener "github.com/gearwheels/go_url_shortener/internal/schemas"
)

// testStoragePath — путь к временному файлу хранилища для тестов (не зависит от CWD)
var testStoragePath string

func init() {
	dir, err := os.MkdirTemp("", "shortener_handler_test")
	if err != nil {
		panic(err)
	}
	testStoragePath = filepath.Join(dir, "store_url.txt")
}

// newTestRouter возвращает роутер с той же цепочкой middleware, что и в main
func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	if config.AppConfig == nil {
		config.Init("localhost:8888", "http://localhost:8000/", testStoragePath, "postgres://shortener:shortener@localhost:5432/shortener")
	}
	if service.Shortener == nil {
		service.Shortener = service.GetService(false, nil)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelError,
		AddSource: false,
	}))

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(logrequest.RequestLogger(logger))
	r.Use(logrequest.RequestDataZip())

	r.Post("/", ShortenHandler)
	r.Post("/api/shorten", JSONShortenHandler)
	r.Get("/{id}", RedirectHandler)

	return r
}

func TestRouterWithMiddleware_ShortenTextPlain(t *testing.T) {
	router := newTestRouter(t)
	url := "https://example.com/middleware-test"

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url))
	req.Header.Set("Content-Type", "text/plain")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	body := strings.TrimSpace(rr.Body.String())
	if !strings.HasPrefix(body, config.AppConfig.BaseURL) {
		t.Errorf("Expected body to start with %s, got %s", config.AppConfig.BaseURL, body)
	}

	id := strings.TrimPrefix(body, config.AppConfig.BaseURL)
	ctx := context.Background()
	original, err := service.Shortener.GetOriginalURL(ctx, id)
	if err != nil {
		t.Errorf("Expected URL to be stored, got error: %v", err)
	}
	if original != url {
		t.Errorf("Expected stored URL %s, got %s", url, original)
	}
}

func TestRouterWithMiddleware_JSONShorten(t *testing.T) {
	router := newTestRouter(t)
	body := `{"url":"https://go.dev/doc"}`

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	var resp schemasShortener.ResponseSchema
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if resp.Result == "" {
		t.Error("Expected result to be set")
	}
}

func TestRouterWithMiddleware_JSONShortenGzipResponse(t *testing.T) {
	router := newTestRouter(t)
	body := `{"url":"https://example.com/gzip"}`

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	if enc := rr.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip when Accept-Encoding: gzip, got %q", enc)
	}

	gr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var resp schemasShortener.ResponseSchema
	if err := json.Unmarshal(decoded, &resp); err != nil {
		t.Fatalf("Decoded body is not valid JSON: %v", err)
	}
	if resp.Result == "" {
		t.Error("Expected result in decoded JSON")
	}
}

func TestRouterWithMiddleware_GzipRequestToJSONShorten(t *testing.T) {
	router := newTestRouter(t)
	plainBody := `{"url":"https://example.com/gzip-request"}`
	gzipped := gzipBytes(t, []byte(plainBody))

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(gzipped))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d for gzipped request body, got %d. Body: %s",
			http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp schemasShortener.ResponseSchema
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response JSON: %v", err)
	}
	if !strings.Contains(resp.Result, config.AppConfig.BaseURL) {
		t.Errorf("Expected result to contain base URL, got %s", resp.Result)
	}
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRouterWithMiddleware_Redirect(t *testing.T) {
	router := newTestRouter(t)
	originalURL := "https://example.com/redirect-middleware"
	ctx := context.Background()
	id, _, err := service.Shortener.ShortenURL(ctx, originalURL, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}


	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status %d, got %d", http.StatusTemporaryRedirect, rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != originalURL {
		t.Errorf("Expected Location %s, got %s", originalURL, loc)
	}
}

func TestRouterWithMiddleware_NotFound(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent-id-12345", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent id, got %d", http.StatusNotFound, rr.Code)
	}
}

func TestRouterWithMiddleware_UnsupportedMediaType(t *testing.T) {
	router := newTestRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://x.com"}`))
	req.Header.Set("Content-Type", "text/plain")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status %d, got %d", http.StatusUnsupportedMediaType, rr.Code)
	}
}
