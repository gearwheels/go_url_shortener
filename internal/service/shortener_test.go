package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "shortener_svc_test")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "store_url.txt")
	config.Init("localhost:8888", "http://localhost:8000/", path, "postgres://shortener:shortener@localhost:5432/shortener")
	Shortener = NewURLShortener()
	os.Exit(m.Run())
}

// TestGenerateID_Uniqueness тестирует уникальность генерируемых ID
func TestGenerateID_Uniqueness(t *testing.T) {
	shortener := NewURLShortener()

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
	shortener := NewURLShortener()

	var wg sync.WaitGroup
	iterations := 100
	ctx := context.Background()

	// Конкурентно добавляем URL
	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			url := fmt.Sprintf("https://example.com/page%d", index)
			_, _, err := shortener.ShortenURL(ctx, url)
			if err != nil {
				t.Errorf("Failed to shorten URL: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что все URL добавлены
	shortener.RLockMu()
	count := len(shortener.store)
	shortener.RUnlockMu()

	if count != iterations {
		t.Errorf("Expected %d URLs in store, got %d", iterations, count)
	}
}

// TestURLShortener_getOriginalURL тестирует получение оригинального URL
func TestURLShortener_GetOriginalURL(t *testing.T) {
	shortener := NewURLShortener()
	ctx := context.Background()

	// Тест 1: Получение несуществующего URL
	_, err := shortener.GetOriginalURL(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existing URL")
	}

	// Тест 2: Получение существующего URL
	testURL := "https://example.com"
	id, _, err := shortener.ShortenURL(ctx, testURL)
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
	shortener := NewURLShortener()
	ctx := context.Background()

	// Тест 1: Создание нового URL
	url1 := "https://example.com"
	id1, _, err := shortener.ShortenURL(ctx, url1)
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	// Проверяем, что URL сохранен
	shortener.RLockMu()
	storedURL, exists := shortener.store[id1]
	shortener.RUnlockMu()

	if !exists {
		t.Error("Expected URL to be stored")
	}

	if storedURL != url1 {
		t.Errorf("Expected stored URL %s, got %s", url1, storedURL)
	}

	// Тест 2: Попытка сократить тот же URL должна вернуть тот же ID
	id2, _, err := shortener.ShortenURL(ctx, url1)
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}
	if id1 != id2 {
		t.Errorf("Expected same ID for same URL, got %s and %s", id1, id2)
	}

	// Тест 3: Создание другого URL
	url3 := "https://example.org"
	id3, _, err := shortener.ShortenURL(ctx, url3)
	if err != nil {
		t.Fatalf("Failed to shorten URL: %v", err)
	}

	if id3 == id1 {
		t.Error("Expected different ID for different URL")
	}

	// Тест 4: Проверка уникальности ID
	shortener.RLockMu()
	count := len(shortener.store)
	shortener.RUnlockMu()

	if count != 2 {
		t.Errorf("Expected 2 URLs in store, got %d", count)
	}
}
