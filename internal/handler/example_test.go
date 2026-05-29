package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	"github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/gearwheels/go_url_shortener/internal/handler"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	"github.com/gearwheels/go_url_shortener/internal/service"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

// initExampleEnv настраивает минимальное окружение для примеров.
func initExampleEnv() {
	if config.AppConfig == nil {
		dir, err := os.MkdirTemp("", "example")
		if err != nil {
			panic(err)
		}
		config.Init(config.InitOptions{
			ServerAddress:   "localhost:8080",
			BaseURL:         "http://localhost:8080/",
			PathStoreURL:    filepath.Join(dir, "store.txt"),
			SecretKeyForJWT: "example-secret",
		})
	}
	if service.Shortener == nil {
		service.Shortener = service.NewShortenerService(repo.NewRepoShortener())
	}
	handler.Auditor = nil
}

// ExampleShortenHandler демонстрирует сокращение URL через POST / (text/plain).
// Успешный запрос возвращает статус 201 Created и короткую ссылку в теле.
func ExampleShortenHandler() {
	initExampleEnv()

	body := strings.NewReader("https://example.com/very/long/path")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "text/plain")
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "user-1"))

	rr := httptest.NewRecorder()
	handler.ShortenHandler(rr, req)

	fmt.Println("Status:", rr.Code)
	fmt.Println("Has body:", len(rr.Body.String()) > 0)

	// Output:
	// Status: 201
	// Has body: true
}

// ExampleShortenHandler_duplicate демонстрирует повторное сокращение того же URL.
// При дубликате сервер возвращает 409 Conflict и тот же shortURL.
func ExampleShortenHandler_duplicate() {
	initExampleEnv()

	originalURL := "https://example.com/duplicate-demo"
	sendRequest := func() *httptest.ResponseRecorder {
		body := strings.NewReader(originalURL)
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "user-dup"))
		rr := httptest.NewRecorder()
		handler.ShortenHandler(rr, req)
		return rr
	}

	first := sendRequest()
	second := sendRequest()

	fmt.Println("First status:", first.Code)
	fmt.Println("Second status:", second.Code)
	fmt.Println("Same body:", first.Body.String() == second.Body.String())

	// Output:
	// First status: 201
	// Second status: 409
	// Same body: true
}

// ExampleJSONShortenHandler демонстрирует сокращение URL через POST /api/shorten (JSON).
// Тело запроса: {"url":"..."}, тело ответа: {"result":"..."}.
func ExampleJSONShortenHandler() {
	initExampleEnv()

	body := strings.NewReader(`{"url":"https://example.com/json-example"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "user-json"))

	rr := httptest.NewRecorder()
	handler.JSONShortenHandler(rr, req)

	fmt.Println("Status:", rr.Code)
	fmt.Println("Content-Type:", rr.Header().Get("Content-Type"))
	fmt.Println("Has result:", strings.Contains(rr.Body.String(), `"result"`))

	// Output:
	// Status: 201
	// Content-Type: application/json
	// Has result: true
}

// ExampleShortenBatchHandler демонстрирует пакетное сокращение POST /api/shorten/batch.
// Принимает массив {correlation_id, original_url}, возвращает массив {correlation_id, short_url}.
func ExampleShortenBatchHandler() {
	initExampleEnv()

	batchBody := `[
		{"correlation_id":"id-1","original_url":"https://example.com/batch-a"},
		{"correlation_id":"id-2","original_url":"https://example.com/batch-b"}
	]`

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch",
		strings.NewReader(batchBody))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "user-batch"))

	rr := httptest.NewRecorder()
	handler.ShortenBatchHandler(rr, req)

	fmt.Println("Status:", rr.Code)
	fmt.Println("Has correlation_id:", strings.Contains(rr.Body.String(), `"correlation_id"`))
	fmt.Println("Has short_url:", strings.Contains(rr.Body.String(), `"short_url"`))

	// Output:
	// Status: 201
	// Has correlation_id: true
	// Has short_url: true
}

// ExampleRedirectHandler демонстрирует редирект GET /{id}.
// Для существующего ID возвращает 307 Temporary Redirect с заголовком Location.
func ExampleRedirectHandler() {
	initExampleEnv()

	// Сначала создаём короткую ссылку
	createReq := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("https://example.com/redirect-target"))
	createReq.Header.Set("Content-Type", "text/plain")
	createReq = createReq.WithContext(logrequest.ContextWithUserID(createReq.Context(), "user-redir"))
	createRR := httptest.NewRecorder()
	handler.ShortenHandler(createRR, createReq)

	// Извлекаем shortID из ответа
	location := createRR.Body.String()
	parts := strings.Split(strings.TrimSpace(location), "/")
	shortID := parts[len(parts)-1]

	// Выполняем редирект
	req := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "user-redir"))
	rr := httptest.NewRecorder()
	handler.RedirectHandler(rr, req)

	fmt.Println("Status:", rr.Code)
	fmt.Println("Has Location:", rr.Header().Get("Location") != "")

	// Output:
	// Status: 307
	// Has Location: true
}

// ExampleUserURL демонстрирует получение всех ссылок пользователя GET /api/user/urls.
func ExampleUserURL() {
	initExampleEnv()

	// Создаём несколько ссылок для одного пользователя
	userID := "user-list"
	for i := 0; i < 2; i++ {
		body := strings.NewReader(fmt.Sprintf("https://example.com/list-%d", i))
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(logrequest.ContextWithUserID(req.Context(), userID))
		handler.ShortenHandler(httptest.NewRecorder(), req)
	}

	// Запрашиваем список
	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), userID))
	rr := httptest.NewRecorder()
	handler.UserURL(rr, req)

	fmt.Println("Status:", rr.Code)
	fmt.Println("Has short_url:", strings.Contains(rr.Body.String(), `"short_url"`))
	fmt.Println("Has original_url:", strings.Contains(rr.Body.String(), `"original_url"`))

	// Output:
	// Status: 200
	// Has short_url: true
	// Has original_url: true
}
