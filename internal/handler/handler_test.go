package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/service"
)

// TestShortenHandler_ContentType тестирует проверку Content-Type
func TestShortenHandler_ContentType(t *testing.T) {

	tests := []struct {
		name        string
		contentType string
		expected    int
	}{
		{"Valid plain text", "text/plain", http.StatusCreated},
		{"Invalid JSON", "application/json", http.StatusUnsupportedMediaType},
		{"Invalid form data", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"Empty", "", http.StatusUnsupportedMediaType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader("https://example.com")
			req := httptest.NewRequest(http.MethodPost, "/", body)
			req.Header.Set("Content-Type", tt.contentType)

			rr := httptest.NewRecorder()
			ShortenHandler(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("Expected status %d for %s, got %d", tt.expected, tt.contentType, rr.Code)
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
			req.Host = "localhost:8080"

			rr := httptest.NewRecorder()
			ShortenHandler(rr, req)

			if rr.Code != http.StatusCreated {
				t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
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
			if !strings.HasPrefix(responseBody, "http://localhost:8080/") {
				t.Errorf("Expected response to start with http://localhost:8080/, got %s", responseBody)
			}

			// Извлекаем ID из ответа
			id := strings.TrimPrefix(responseBody, "http://localhost:8080/")

			// Проверяем, что URL сохранен правильно
			storedURL, exists := service.Shortener.GetOriginalURL(id)
			if !exists {
				t.Error("Expected URL to be stored")
			}

			if storedURL != tc.expected {
				t.Errorf("Expected stored URL %s, got %s", tc.expected, storedURL)
			}
		})
	}
}

// TestShortenHandler_DuplicateURL тестирует обработку дублирующихся URL
func TestShortenHandler_DuplicateURL(t *testing.T) {
	url := "https://example.com/unique"

	// Первый запрос
	body1 := strings.NewReader(url)
	req1 := httptest.NewRequest(http.MethodPost, "/", body1)
	req1.Header.Set("Content-Type", "text/plain")
	req1.Host = "localhost:8080"

	rr1 := httptest.NewRecorder()
	ShortenHandler(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("First request failed with status %d", rr1.Code)
	}

	shortURL1 := rr1.Body.String()
	id1 := strings.TrimPrefix(shortURL1, "http://localhost:8080/")

	// Второй запрос с тем же URL
	body2 := strings.NewReader(url)
	req2 := httptest.NewRequest(http.MethodPost, "/", body2)
	req2.Header.Set("Content-Type", "text/plain")
	req2.Host = "localhost:8080"

	rr2 := httptest.NewRecorder()
	ShortenHandler(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Fatalf("Second request failed with status %d", rr2.Code)
	}

	shortURL2 := rr2.Body.String()
	id2 := strings.TrimPrefix(shortURL2, "http://localhost:8080/")

	// ID должны быть одинаковыми для одинаковых URL
	if id1 != id2 {
		t.Errorf("Expected same ID for duplicate URL, got %s and %s", id1, id2)
	}

	// Проверяем, что в хранилище только одна запись
	service.Shortener.Mu.RLock()
	count := service.Shortener.GetLenStore()
	service.Shortener.Mu.RUnlock()

	if count != 1 {
		t.Errorf("Expected 1 URL in store for duplicates, got %d", count)
	}
}

// TestMainHandler_Integration тестирует полный сценарий работы
func TestMainHandler_Integration(t *testing.T) {
	// создаем короткий url
	url := "https://go.dev/dl/"
	body := strings.NewReader(url)

	req1 := httptest.NewRequest(http.MethodPost, "/", body)

	req1.Header.Set("Content-Type", "text/plain")

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

	// Сначала создаем короткий URL
	originalURL := "https://example.com/redirect-test"
	id := service.Shortener.ShortenURL(originalURL)

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

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d for root path, got %d", http.StatusOK, rr.Code)
	}

	expectedMessage := "Send POST request with URL in body as text/plain to shorten URL"
	if !strings.Contains(rr.Body.String(), expectedMessage) {
		t.Errorf("Expected message '%s', got '%s'", expectedMessage, rr.Body.String())
	}
}
