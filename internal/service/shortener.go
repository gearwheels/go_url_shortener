package service

import (
	"crypto/rand"
	"encoding/base64"
	"log"
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

func (us *URLShortener) ShortenURL(originalURL string) string {
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

func (us *URLShortener) GetOriginalURL(id string) (string, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	url, exists := us.store[id]
	return url, exists
}

var Shortener = NewURLShortener()