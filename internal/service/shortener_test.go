package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "shortener_svc_test")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "store_url.txt")
	config.Init("localhost:8888", "http://localhost:8000/", path, "postgres://shortener:shortener@localhost:5432/shortener", "test-secret", "", "")
	Shortener = NewShortenerService(repo.NewRepoShortener())
	os.Exit(m.Run())
}

func newTestService() URLShortenerInterface {
	return NewShortenerService(repo.NewRepoShortener())
}

// TestGenerateID_Uniqueness тестирует уникальность генерируемых ID
func TestGenerateID_Uniqueness(t *testing.T) {
	shortener := newTestService()

	iterations := 1000
	ids := make(map[string]bool)

	for i := 0; i < iterations; i++ {
		id := shortener.GenerateID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != iterations {
		t.Errorf("Expected %d unique IDs, got %d", iterations, len(ids))
	}
}

// TestConcurrentAccess тестирует конкурентный доступ к хранилищу
func TestConcurrentAccess(t *testing.T) {
	shortener := newTestService()

	var wg sync.WaitGroup
	iterations := 100
	ctx := context.Background()

	// Конкурентно добавляем URL
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			url := fmt.Sprintf("https://example.com/page%d", index)
			id, inserted, err := shortener.ShortenURL(ctx, url, "")
			if err != nil {
				t.Errorf("Failed to shorten URL: %v", err)
				return
			}
			if id == "" {
				t.Error("Expected non-empty ID")
			}
			if !inserted {
				// для разных URL вставка должна быть новой
				t.Error("Expected inserted=true for new URL")
			}
		}(i)
	}

	wg.Wait()
}

// TestURLShortener_getOriginalURL тестирует получение оригинального URL
func TestURLShortener_GetOriginalURL(t *testing.T) {
	shortener := newTestService()
	ctx := context.Background()

	// Тест 1: Получение несуществующего URL
	_, _, err := shortener.GetOriginalURL(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existing URL")
	}

	// Тест 2: Получение существующего URL
	testURL := "https://example.com"
	id, _, err := shortener.ShortenURL(ctx, testURL, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	storedURL, _, err := shortener.GetOriginalURL(ctx, id)
	if err != nil {
		t.Errorf("Expected existing URL to exist, got error: %v", err)
	}
	if storedURL != testURL {
		t.Errorf("Expected URL %s, got %s", testURL, storedURL)
	}
}

// TestURLShortener_shortenURL тестирует сокращение URL
func TestURLShortener_ShortenURL(t *testing.T) {
	shortener := newTestService()
	ctx := context.Background()

	// Тест 1: Создание нового URL
	url1 := "https://example.com"
	id1, inserted1, err := shortener.ShortenURL(ctx, url1, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}
	if !inserted1 {
		t.Error("Expected inserted=true for first URL")
	}

	// Проверяем, что URL сохранен через GetOriginalURL
	storedURL, _, err := shortener.GetOriginalURL(ctx, id1)
	if err != nil {
		t.Fatalf("Expected URL to be stored, got error: %v", err)
	}
	if storedURL != url1 {
		t.Errorf("Expected stored URL %s, got %s", url1, storedURL)
	}

	// Тест 2: Попытка сократить тот же URL должна вернуть тот же ID
	id2, inserted2, err := shortener.ShortenURL(ctx, url1, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}
	if id1 != id2 {
		t.Errorf("Expected same ID for same URL, got %s and %s", id1, id2)
	}
	if inserted2 {
		t.Error("Expected inserted=false for duplicate URL")
	}

	// Тест 3: Создание другого URL
	url3 := "https://example.org"
	id3, inserted3, err := shortener.ShortenURL(ctx, url3, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	if id3 == id1 {
		t.Error("Expected different ID for different URL")
	}
	if !inserted3 {
		t.Error("Expected inserted=true for new second URL")
	}
}

func TestGenerateUniqueID_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := GenerateUniqueID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate GenerateUniqueID at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
		if len(id) != 10 {
			t.Fatalf("expected length 10, got %d (%s)", len(id), id)
		}
	}
}

func TestPadBase62(t *testing.T) {
	// padBase62 always returns partLen (5) characters
	cases := []uint64{0, 1, 61, 62, 62*62*62*62*62 - 1}
	for _, val := range cases {
		got := padBase62(val, partLen)
		if len(got) != partLen {
			t.Errorf("padBase62(%d): len=%d, want %d", val, len(got), partLen)
		}
	}
}

func TestGetAllShortenerURL(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _, err := svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/u%d", i), "user-a")
		if err != nil {
			t.Fatalf("ShortenURL: %v", err)
		}
	}
	if _, _, err := svc.ShortenURL(ctx, "https://other.com", "user-b"); err != nil {
		t.Fatalf("ShortenURL user-b: %v", err)
	}

	urls, err := svc.GetAllShortenerURL(ctx, "user-a")
	if err != nil {
		t.Fatalf("GetAllShortenerURL: %v", err)
	}
	if len(urls) != 5 {
		t.Errorf("expected 5 URLs for user-a, got %d", len(urls))
	}

	urls2, err := svc.GetAllShortenerURL(ctx, "user-b")
	if err != nil {
		t.Fatalf("GetAllShortenerURL user-b: %v", err)
	}
	if len(urls2) != 1 {
		t.Errorf("expected 1 URL for user-b, got %d", len(urls2))
	}
}

func TestShortenURLBatch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	batch := []schemasshortener.RequestBatchURLSchema{
		{CorrelationID: "1", OriginalURL: "https://example.com/a"},
		{CorrelationID: "2", OriginalURL: "https://example.com/b"},
		{CorrelationID: "3", OriginalURL: "example.com/c"},
	}

	resp, err := svc.ShortenURLBatch(ctx, batch, "u1")
	if err != nil {
		t.Fatalf("ShortenURLBatch: %v", err)
	}
	if len(resp) != 3 {
		t.Fatalf("expected 3 responses, got %d", len(resp))
	}
	for i, r := range resp {
		if r.ShortURL == "" {
			t.Errorf("response[%d]: empty short_url", i)
		}
	}

	// empty batch returns empty slice without error
	resp2, err := svc.ShortenURLBatch(ctx, nil, "u1")
	if err != nil {
		t.Fatalf("empty batch error: %v", err)
	}
	if len(resp2) != 0 {
		t.Errorf("expected 0 responses for empty batch, got %d", len(resp2))
	}
}

func TestMarkOnDeleteBatch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	id1, _, err := svc.ShortenURL(ctx, "https://example.com/del1", "u1")
	if err != nil {
		t.Fatalf("ShortenURL del1: %v", err)
	}
	id2, _, err := svc.ShortenURL(ctx, "https://example.com/del2", "u1")
	if err != nil {
		t.Fatalf("ShortenURL del2: %v", err)
	}

	if err := svc.MarkOnDeleteBatch(ctx, "u1", []string{id1, id2}); err != nil {
		t.Fatalf("MarkOnDeleteBatch: %v", err)
	}

	_, deleted, err := svc.GetOriginalURL(ctx, id1)
	if err != nil {
		t.Fatalf("GetOriginalURL: %v", err)
	}
	if !deleted {
		t.Error("expected id1 to be marked deleted")
	}

	// empty list is a no-op
	if err := svc.MarkOnDeleteBatch(ctx, "u1", nil); err != nil {
		t.Fatalf("MarkOnDeleteBatch nil: %v", err)
	}
}

func TestIsShortURLCollision(t *testing.T) {
	if isShortURLCollision(nil) {
		t.Error("nil should not be a collision")
	}
	if !isShortURLCollision(errors.New("short_url already exists")) {
		t.Error("expected collision for 'short_url already exists'")
	}
	if isShortURLCollision(errors.New("some other error")) {
		t.Error("other errors should not be a collision")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("nil error should not be a unique violation")
	}
	if isUniqueViolation(errors.New("some error")) {
		t.Error("generic error should not be a unique violation")
	}
}

func TestGetService_InMemory(t *testing.T) {
	svc := GetService(false, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	ctx := context.Background()
	id, inserted, err := svc.ShortenURL(ctx, "https://getservice.test/", "u")
	if err != nil {
		t.Fatalf("ShortenURL via GetService: %v", err)
	}
	if id == "" || !inserted {
		t.Error("expected valid id and inserted=true")
	}
}
