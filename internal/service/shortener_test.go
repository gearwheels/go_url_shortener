package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "shortener_svc_test")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "store_url.txt")
	config.Init("localhost:8888", "http://localhost:8000/", path, "postgres://shortener:shortener@localhost:5432/shortener")
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
	_, err := shortener.GetOriginalURL(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existing URL")
	}

	// Тест 2: Получение существующего URL
	testURL := "https://example.com"
	id, _, err := shortener.ShortenURL(ctx, testURL, "")
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	storedURL, err := shortener.GetOriginalURL(ctx, id)
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
	storedURL, err := shortener.GetOriginalURL(ctx, id1)
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
