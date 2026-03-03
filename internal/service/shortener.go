package service

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	config "github.com/gearwheels/go_url_shortener/internal/config"
	"github.com/jmoiron/sqlx"
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

func (us *URLShortener) GenerateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (us *URLShortener) ShortenURL(ctx context.Context, originalURL string) (string, error) {
	us.mu.Lock()
	defer us.mu.Unlock()

	// Проверяем, есть ли уже такой URL в хранилище
	for id, url := range us.store {
		if url == originalURL {
			return id, nil // Возвращаем существующий ID
		}
	}
	// Генерируем уникальный ID
	var id string
	for {
		id = us.GenerateID()
		if _, exists := us.store[id]; !exists {
			break
		}
	}
	// Сохраняем в хранилище
	us.store[id] = originalURL
	us.UpdateFile("\"" + id + "\": \"" + originalURL + "\"")
	slog.Info("Shortened URL: " + id + " -> " + originalURL)
	return id, nil
}

func (us *URLShortener) GetOriginalURL(ctx context.Context, id string) (string, error) {
	us.mu.RLock()
	defer us.mu.RUnlock()

	url, exists := us.store[id]
	if !exists {
		return "", errors.New("short URL not found")
	}
	return url, nil
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

var Shortener URLShortenerInterface

func GetService(pgExist bool, db interface{}) URLShortenerInterface {
	if pgExist {
		if dbConn, ok := db.(*sqlx.DB); ok {
			return NewURLPostgresRepository(dbConn)
		}
		// return nil
	}
	return NewURLShortener()
}

