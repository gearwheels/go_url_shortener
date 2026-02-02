package service

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	config "github.com/gearwheels/go_url_shortener/internal/config"
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

func (us *URLShortener) RLockMu() {
	us.mu.RLock()
}

func (us *URLShortener) RUnlockMu() {
	us.mu.RUnlock()
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
	Shortener.UpdateFile("\"" + id + "\": \"" + originalURL + "\"")
	slog.Info("Shortened URL: " + id + " -> " + originalURL)
	return id
}

func (us *URLShortener) GetOriginalURL(id string) (string, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	url, exists := us.store[id]
	return url, exists
}

func (us *URLShortener) GetLenStore() int {
	return len(us.store)
}

func (us *URLShortener) PrintStore() []byte {
	jsonStr, err := json.Marshal(us.store)
	if err != nil {
		slog.Error(err.Error())
		return nil
	}
	return jsonStr
}

func (us *URLShortener) FreeStore() {
	us.store = make(map[string]string)
}

func (us *URLShortener) UpdateFile(data string) error {
	file, err := os.OpenFile(config.AppConfig.PathStoreURL, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
		// return nil, err
	}
	defer file.Close()

	info, err := os.Stat(config.AppConfig.PathStoreURL)
	if err != nil {
		slog.Error("Ошибка: " + err.Error() + "\n")
		panic(err)
	}
	// data := us.PrintStore()
	writer := bufio.NewWriter(file)
	// Проверяем размер файла
	if info.Size() != 0 {
		// добавляем запятую в конце строки
		if err := writer.WriteByte(','); err != nil {
			return err
		}
		// добавляем перенос строки
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	if _, err := writer.WriteString(data); err != nil {
		return err
	}
	return writer.Flush()
}

func (us *URLShortener) ExtractFromFile() error {
	file, err := os.OpenFile(config.AppConfig.PathStoreURL, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
		// return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var content strings.Builder

	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	fmt.Println("Содержимое файла:")
	fmt.Println(content.String())
	fmt.Printf("Размер: %d байт\n", content.Len())

	if len(content.String()) != 0 {
		us.FreeStore()
		jsonStr := "{" + content.String() + "}"
		err = json.Unmarshal([]byte(jsonStr), &us.store)
		if err != nil {
			return err
		}
	} else {
		slog.Info("Storage file is empty")
	}
	return nil
}

var Shortener = NewURLShortener()
