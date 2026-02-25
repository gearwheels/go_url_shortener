package repository

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"

	config "github.com/gearwheels/go_url_shortener/internal/config"
)

// URLShortener — in-memory репозиторий.
// Репозиторий отвечает только за хранение/чтение данных и НЕ содержит бизнес-логики
// (генерации ID, правил дедупликации и т.п.).
type URLShortener struct {
	mu sync.RWMutex

	nextID  int64
	byID    map[int64]URL
	byURL   map[string]int64
	byShort map[string]int64
}

func NewRepoShortener() *URLShortener {
	return &URLShortener{
		nextID:  1,
		byID:    make(map[int64]URL),
		byURL:   make(map[string]int64),
		byShort: make(map[string]int64),
	}
}

var _ ShortenerRepository = (*URLShortener)(nil)

func (r *URLShortener) Create(ctx context.Context, u URL) (shortCode string, inserted bool, err error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()

	if u.URL == "" || u.ShortURL == "" {
		return "", false, errors.New("url and short_url are required")
	}

	// url уже существует → вернуть существующий short_url
	if id, ok := r.byURL[u.URL]; ok {
		existing := r.byID[id]
		return existing.ShortURL, false, nil
	}

	// short_url уже занят другим url → коллизия (service должен сгенерировать другой ID)
	if id, ok := r.byShort[u.ShortURL]; ok {
		existing := r.byID[id]
		if existing.URL != u.URL {
			return "", false, errors.New("short_url already exists")
		}
		return existing.ShortURL, false, nil
	}

	u.ID = r.nextID
	r.nextID++

	r.byID[u.ID] = u
	r.byURL[u.URL] = u.ID
	r.byShort[u.ShortURL] = u.ID

	_ = r.UpdateFile("\"" + u.ShortURL + "\": \"" + u.URL + "\"")
	return u.ShortURL, true, nil
}

func (r *URLShortener) GetByID(ctx context.Context, id int64) (URL, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return URL{}, nil
}

func (r *URLShortener) GetByURL(ctx context.Context, url string) (URL, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.byURL[url]; ok {
		return r.byID[id], nil
	}
	return URL{}, nil
}

func (r *URLShortener) GetByShortURL(ctx context.Context, shortURL string) (URL, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.byShort[shortURL]; ok {
		return r.byID[id], nil
	}
	return URL{}, nil
}

func (r *URLShortener) Delete(ctx context.Context, id int64) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.byID[id]
	if !ok {
		return sql.ErrNoRows
	}
	delete(r.byID, id)
	delete(r.byURL, u.URL)
	delete(r.byShort, u.ShortURL)
	return nil
}

func (r *URLShortener) List(ctx context.Context) ([]URL, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]int64, 0, len(r.byID))
	for id := range r.byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	out := make([]URL, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.byID[id])
	}
	return out, nil
}

// Вспомогательные методы для тестов и поддержки файла‑хранилища.

func (r *URLShortener) RLockMu() {
	r.mu.RLock()
}

func (r *URLShortener) RUnlockMu() {
	r.mu.RUnlock()
}

func (r *URLShortener) GetLenStore() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

func (r *URLShortener) PrintStore() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	flat := make(map[string]string, len(r.byID))
	for _, u := range r.byID {
		flat[u.ShortURL] = u.URL
	}
	jsonStr, err := json.Marshal(flat)
	if err != nil {
		return nil
	}
	return jsonStr
}

func (r *URLShortener) FreeStore() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID = 1
	r.byID = make(map[int64]URL)
	r.byURL = make(map[string]int64)
	r.byShort = make(map[string]int64)
}

func (r *URLShortener) UpdateFile(data string) error {
	file, err := os.OpenFile(config.AppConfig.PathStoreURL, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := os.Stat(config.AppConfig.PathStoreURL)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	if info.Size() != 0 {
		if err := writer.WriteByte(','); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	if _, err := writer.WriteString(data); err != nil {
		return err
	}
	return writer.Flush()
}

func (r *URLShortener) ExtractFromFile() error {
	file, err := os.OpenFile(config.AppConfig.PathStoreURL, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var content strings.Builder

	for scanner.Scan() {
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	if len(content.String()) != 0 {
		r.FreeStore()
		jsonStr := "{" + content.String() + "}"
		var flat map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &flat); err != nil {
			return err
		}
		r.mu.Lock()
		for shortID, originalURL := range flat {
			u := URL{ID: r.nextID, URL: originalURL, ShortURL: shortID}
			r.byID[u.ID] = u
			r.byURL[u.URL] = u.ID
			r.byShort[u.ShortURL] = u.ID
			r.nextID++
		}
		r.mu.Unlock()
	}
	return nil
}

