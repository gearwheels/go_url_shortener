package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	"github.com/gearwheels/go_url_shortener/internal/service"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

// TestShortenHandler_ContentType тестирует проверку Content-Type
func TestShortenHandler_ContentType(t *testing.T) {

	tests := []struct {
		name        string
		contentType string
		expectedOk  []int
	}{
		{"Valid plain text", "text/plain", []int{http.StatusCreated, http.StatusConflict}},
		{"Invalid JSON", "application/json", []int{http.StatusUnsupportedMediaType}},
		{"Invalid form data", "application/x-www-form-urlencoded", []int{http.StatusUnsupportedMediaType}},
		{"Empty", "", []int{http.StatusUnsupportedMediaType}},
	}

	if config.AppConfig == nil {
		fmt.Println("AppConfig don't init")
		config.Init(config.InitOptions{
			ServerAddress:   "localhost:8888",
			BaseURL:         "http://localhost:8000/",
			PathStoreURL:    "./storage/store_url.txt",
			DatabaseDsn:     "postgres://shortener:shortener@localhost:5432/shortener",
			SecretKeyForJWT: "test-secret",
		})
	} else {
		fmt.Println("AppConfig has been init-ed")
	}
	if service.Shortener == nil {
		service.Shortener = service.GetService(false, nil)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader("https://example.com")
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", tt.contentType)
			if tt.contentType == "text/plain" {
				req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))
			}

			rr := httptest.NewRecorder()
			ShortenHandler(rr, req)

			ok := false
			for _, code := range tt.expectedOk {
				if rr.Code == code {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Expected status one of %v for %s, got %d", tt.expectedOk, tt.contentType, rr.Code)
			}
		})
	}
}

// TestShortenHandler_EmptyBody тестирует обработку пустого тела запроса
func TestShortenHandler_EmptyBody(t *testing.T) {

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")

	rr := httptest.NewRecorder()
	ShortenHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty body, got %d", http.StatusBadRequest, rr.Code)
	}

	expectedError := "URL cannot be empty"
	if !strings.Contains(rr.Body.String(), expectedError) {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, rr.Body.String())
	}
}

// TestShortenHandler_ValidURL тестирует успешное сокращение URL
func TestShortenHandler_ValidURL(t *testing.T) {

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Full HTTPS URL", "https://example.com/path", "https://example.com/path"},
		{"Full HTTP URL", "http://example.com", "http://example.com"},
		{"URL without scheme", "example.com", "http://example.com"},
		{"URL with path", "example.com/path/to/resource", "http://example.com/path/to/resource"},
		{"URL with query", "example.com?query=test", "http://example.com?query=test"},
		{"URL with fragment", "example.com#section", "http://example.com#section"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.NewReader(tc.input)
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", "text/plain")
			req.Host = config.AppConfig.ServerAddress
			req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))

			rr := httptest.NewRecorder()
			ShortenHandler(rr, req)

			// 201 — новая запись, 409 — URL уже был сокращён (дубликат)
			if rr.Code != http.StatusCreated && rr.Code != http.StatusConflict {
				t.Errorf("Expected status %d or %d, got %d", http.StatusCreated, http.StatusConflict, rr.Code)
			}

			// Проверяем заголовки
			contentType := rr.Header().Get("Content-Type")
			if contentType != "text/plain" {
				t.Errorf("Expected Content-Type text/plain, got %s", contentType)
			}

			location := rr.Header().Get("Location")
			if location == "" {
				t.Error("Expected Location header to be set")
			}

			// Проверяем тело ответа
			responseBody := rr.Body.String()
			if !strings.HasPrefix(responseBody, config.AppConfig.BaseURL) {
				t.Errorf("Expected response to start with %s, got %s", config.AppConfig.BaseURL, responseBody)
			}

			// Извлекаем ID из ответа
			id := strings.TrimPrefix(responseBody, config.AppConfig.BaseURL)
			// Проверяем, что URL сохранен правильно
			reqCtx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
			storedURL, _, err := service.Shortener.GetOriginalURL(reqCtx, id)

			if err != nil {
				t.Errorf("Expected URL to be stored, got error: %v", err)
			}

			if storedURL != tc.expected {
				t.Errorf("Expected stored URL %s, got %s", tc.expected, storedURL)
			}
		})
	}
}

// TestShortenHandler_DuplicateURL тестирует обработку дублирующихся URL
func TestShortenHandler_DuplicateURL(t *testing.T) {
	service.Shortener = service.GetService(false, nil)
	url := "https://example.com/unique"
	ctxWithUser := logrequest.ContextWithUserID(context.Background(), "test-user-id")

	// Первый запрос
	body1 := strings.NewReader(url)
	req1 := httptest.NewRequest(http.MethodPost, "/", body1)
	req1.Header.Set("Content-Type", "text/plain")
	req1.Host = "localhost:8080"
	req1 = req1.WithContext(ctxWithUser)

	rr1 := httptest.NewRecorder()
	ShortenHandler(rr1, req1)

	if rr1.Code != http.StatusCreated && rr1.Code != http.StatusConflict {
		t.Fatalf("First request expected status %d or %d, got %d", http.StatusCreated, http.StatusConflict, rr1.Code)
	}

	shortURL1 := rr1.Body.String()
	id1 := strings.TrimPrefix(shortURL1, "http://localhost:8080/")

	// Второй запрос с тем же URL — должен вернуть 409 Conflict и тот же short URL
	body2 := strings.NewReader(url)
	req2 := httptest.NewRequest(http.MethodPost, "/", body2)
	req2.Header.Set("Content-Type", "text/plain")
	req2.Host = "localhost:8080"
	req2 = req2.WithContext(ctxWithUser)

	rr2 := httptest.NewRecorder()
	ShortenHandler(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("Second request (duplicate) expected status %d, got %d", http.StatusConflict, rr2.Code)
	}

	shortURL2 := rr2.Body.String()
	id2 := strings.TrimPrefix(shortURL2, "http://localhost:8080/")

	// ID должны быть одинаковыми для одинаковых URL
	if id1 != id2 {
		t.Errorf("Expected same ID for duplicate URL, got %s and %s", id1, id2)
	}
}

// TestMainHandler_Integration тестирует полный сценарий работы
func TestMainHandler_Integration(t *testing.T) {
	// создаем короткий url
	url := "https://go.dev/dl/"
	body := strings.NewReader(url)

	req1 := httptest.NewRequest(http.MethodPost, "/", body)
	req1.Header.Set("Content-Type", "text/plain")
	req1 = req1.WithContext(logrequest.ContextWithUserID(req1.Context(), "test-user-id"))

	w := httptest.NewRecorder()
	ShortenHandler(w, req1)
	resp1 := w.Result()

	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp1.StatusCode)
	}

	respBody, err := io.ReadAll(resp1.Body)
	if err != nil {
		t.Fatal(err)
	}

	shortURL := string(respBody)
	parts := strings.Split(shortURL, "/")
	id := parts[3]

	// получаем исходный урл
	req2 := httptest.NewRequest(http.MethodGet, "/"+id, nil)

	w2 := httptest.NewRecorder()
	RedirectHandler(w2, req2)

	resp2 := w2.Result()

	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Expected status %d, got %d", http.StatusTemporaryRedirect, resp2.StatusCode)
	}

	location := resp2.Header.Get("Location")
	if location != url {
		t.Errorf("Expected Location %s, got %s", url, location)
	}
}

// TestShortenHandler_InvalidPath тестирует обработку некорректного пути
func TestShortenHandler_InvalidPath(t *testing.T) {

	tests := []struct {
		path     string
		expected int
	}{
		{"/api", http.StatusNotFound},
		{"/shorten", http.StatusNotFound},
		{"/url/", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			body := strings.NewReader("https://example.com")
			req := httptest.NewRequest(http.MethodPost, tt.path, body)
			req.Header.Set("Content-Type", "text/plain")

			rr := httptest.NewRecorder()
			ShortenHandler(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("For path %s expected status %d, got %d", tt.path, tt.expected, rr.Code)
			}
		})
	}
}

// TestRedirectHandler_NotFound тестирует перенаправление для несуществующего ID
func TestRedirectHandler_NotFound(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()

	RedirectHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status %d for non-existent ID, got %d", http.StatusNotFound, rr.Code)
	}
}

// TestRedirectHandler_Success тестирует успешное перенаправление
func TestRedirectHandler_Success(t *testing.T) {
	if service.Shortener == nil {
		service.Shortener = service.GetService(false, nil)
	}

	// Сначала создаем короткий URL
	originalURL := "https://example.com/redirect-test"
	ctx := context.Background()
	id, _, err := service.Shortener.ShortenURL(ctx, originalURL, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	// Тестируем перенаправление
	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rr := httptest.NewRecorder()

	RedirectHandler(rr, req)

	if rr.Code != http.StatusTemporaryRedirect {
		t.Errorf("Expected status %d, got %d", http.StatusTemporaryRedirect, rr.Code)
	}

	location := rr.Header().Get("Location")
	if location != originalURL {
		t.Errorf("Expected Location %s, got %s", originalURL, location)
	}
}

// TestRedirectHandler_RootPath тестирует корневой путь GET
func TestRedirectHandler_RootPath(t *testing.T) {

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	RedirectHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for root path, got %d", http.StatusBadRequest, rr.Code)
	}

	expectedMessage := "Send POST request with URL in body as text/plain to shorten URL"
	if !strings.Contains(rr.Body.String(), expectedMessage) {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, rr.Body.String())
	}
}

// TestJSONShortenHandler_ContentType тестирует проверку Content-Type для JSON handler
func TestJSONShortenHandler_ContentType(t *testing.T) {

	tests := []struct {
		name        string
		contentType string
		expectedOk  []int
	}{
		{"Valid JSON", "application/json", []int{http.StatusCreated, http.StatusConflict}},
		{"Invalid plain text", "text/plain", []int{http.StatusUnsupportedMediaType}},
		{"Invalid form data", "application/x-www-form-urlencoded", []int{http.StatusUnsupportedMediaType}},
		{"Empty", "", []int{http.StatusUnsupportedMediaType}},
	}

	body := `{"url":"https://example.com"}`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
			req.Header.Set("Content-Type", tt.contentType)
			if tt.contentType == "application/json" {
				req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))
			}

			rr := httptest.NewRecorder()
			JSONShortenHandler(rr, req)

			ok := false
			for _, code := range tt.expectedOk {
				if rr.Code == code {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("Expected status one of %v for Content-Type %q, got %d", tt.expectedOk, tt.contentType, rr.Code)
			}
		})
	}
}

// TestJSONShortenHandler_InvalidJSON тестирует обработку невалидного JSON
func TestJSONShortenHandler_InvalidJSON(t *testing.T) {

	tests := []struct {
		name string
		body string
	}{
		{"Not JSON", "not json at all"},
		{"Malformed JSON", `{"url":`},
		{"Wrong structure", `{"link":"https://example.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			JSONShortenHandler(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d for invalid JSON, got %d", http.StatusBadRequest, rr.Code)
			}
		})
	}
}

// TestJSONShortenHandler_EmptyURL тестирует обработку пустого URL в JSON
func TestJSONShortenHandler_EmptyURL(t *testing.T) {

	body := `{"url":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	JSONShortenHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d for empty URL, got %d", http.StatusBadRequest, rr.Code)
	}
	expectedError := "URL cannot be empty"
	if !strings.Contains(rr.Body.String(), expectedError) {
		t.Errorf("Expected error message containing %q, got %q", expectedError, rr.Body.String())
	}
}

// TestJSONShortenHandler_ValidURL тестирует успешное сокращение URL через JSON API
func TestJSONShortenHandler_ValidURL(t *testing.T) {

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"Full HTTPS URL", "https://example.com/path", "https://example.com/path"},
		{"Full HTTP URL", "http://example.com", "http://example.com"},
		{"URL without scheme", "example.com", "http://example.com"},
		{"URL with path", "example.com/path/to/resource", "http://example.com/path/to/resource"},
		{"URL with query", "example.com?query=test", "http://example.com?query=test"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqBody := map[string]string{"url": tc.input}
			bodyBytes, err := json.Marshal(reqBody)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(string(bodyBytes)))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))

			rr := httptest.NewRecorder()
			JSONShortenHandler(rr, req)

			// 201 — новая запись, 409 — URL уже был сокращён (дубликат)
			if rr.Code != http.StatusCreated && rr.Code != http.StatusConflict {
				t.Errorf("Expected status %d or %d, got %d. Body: %s", http.StatusCreated, http.StatusConflict, rr.Code, rr.Body.String())
			}

			contentType := rr.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Expected Content-Type application/json, got %s", contentType)
			}

			var resp schemasshortener.ResponseSchema
			if unmarshalErr := json.Unmarshal(rr.Body.Bytes(), &resp); unmarshalErr != nil {
				t.Fatalf("Failed to unmarshal response: %v", unmarshalErr)
			}

			if resp.Result == "" {
				t.Error("Expected result field to be set")
			}
			if !strings.HasPrefix(resp.Result, config.AppConfig.BaseURL) {
				t.Errorf("Expected result to start with %s, got %s", config.AppConfig.BaseURL, resp.Result)
			}

			id := strings.TrimPrefix(resp.Result, config.AppConfig.BaseURL)
			reqCtx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
			storedURL, _, err := service.Shortener.GetOriginalURL(reqCtx, id)
			if err != nil {
				t.Errorf("Expected URL to be stored, got error: %v", err)
			}
			if storedURL != tc.expected {
				t.Errorf("Expected stored URL %s, got %s", tc.expected, storedURL)
			}
		})
	}
}

// TestJSONShortenHandler_ResponseFormat тестирует формат JSON-ответа
func TestJSONShortenHandler_ResponseFormat(t *testing.T) {

	body := `{"url":"https://go.dev/"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))

	rr := httptest.NewRecorder()
	JSONShortenHandler(rr, req)

	if rr.Code != http.StatusCreated && rr.Code != http.StatusConflict {
		t.Fatalf("Expected status %d or %d, got %d", http.StatusCreated, http.StatusConflict, rr.Code)
	}

	var resp schemasshortener.ResponseSchema
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}

	if resp.Result == "" {
		t.Error("Expected result field to be non-empty")
	}
	if !strings.HasPrefix(resp.Result, config.AppConfig.BaseURL) {
		t.Errorf("Expected result to be short URL starting with base, got %s", resp.Result)
	}
}

// TestShortenBatchHandler тестирует батчевое сокращение URL
func TestShortenBatchHandler(t *testing.T) {
	if config.AppConfig == nil {
		config.Init(config.InitOptions{
			ServerAddress:   "localhost:8080",
			BaseURL:         "http://localhost:8080/",
			PathStoreURL:    "./storage/store_url.txt",
			DatabaseDsn:     "postgres://shortener:shortener@localhost:5432/shortener",
			SecretKeyForJWT: "test-secret",
		})
	}
	service.Shortener = service.NewShortenerService(repo.NewRepoShortener())

	body := `[
		{"correlation_id": "req-1", "original_url": "https://example.com/batch1"},
		{"correlation_id": "req-2", "original_url": "https://example.com/batch2"}
	]`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "test-user-id"))

	rr := httptest.NewRecorder()
	ShortenBatchHandler(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d. Body: %s", http.StatusCreated, rr.Code, rr.Body.String())
	}

	var resp []schemasshortener.ResponseBatchURLSchema
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Response is not valid JSON: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("Expected 2 items in response, got %d", len(resp))
	}

	ctx := httptest.NewRequest(http.MethodGet, "/", nil).Context()
	for i, item := range resp {
		if item.CorrelationID == "" || item.ShortURL == "" {
			t.Errorf("item[%d]: empty correlation_id or short_url", i)
		}
		if !strings.HasPrefix(item.ShortURL, config.AppConfig.BaseURL) {
			t.Errorf("item[%d]: short_url should start with base: %s", i, item.ShortURL)
		}
		id := strings.TrimPrefix(item.ShortURL, config.AppConfig.BaseURL)
		id = strings.Trim(id, "/")
		original, _, err := service.Shortener.GetOriginalURL(ctx, id)
		if err != nil {
			t.Errorf("item[%d]: GetOriginalURL(%s): %v", i, id, err)
		}
		expected := []string{"https://example.com/batch1", "https://example.com/batch2"}
		if i < len(expected) && original != expected[i] {
			t.Errorf("item[%d]: expected original %s, got %s", i, expected[i], original)
		}
	}
}
