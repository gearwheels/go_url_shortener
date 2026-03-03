package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gearwheels/go_url_shortener/internal/config"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	repo "github.com/gearwheels/go_url_shortener/repositories"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

var (
    startPrefix string   // префикс, зависящий от времени запуска
    counter     uint64   // атомарный счётчик
)

const (
    alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" // base62
    partLen  = 5
)
// генерация неповторяющихся строк для батчинга
// init инициализирует префикс запуска и счётчик
func init() {
    nano := time.Now().UnixNano()
    b := make([]byte, 8)
    binary.LittleEndian.PutUint64(b, uint64(nano))
    startPrefix = encodeBase62(uint64(nano))
    for len(startPrefix) < partLen {
        startPrefix = "0" + startPrefix
    }
    if len(startPrefix) > partLen {
        startPrefix = startPrefix[:partLen]
    }
    counter = 0
}

func encodeBase62(val uint64) string {
    if val == 0 {
        return "0"
    }
    const base = uint64(len(alphabet))
    var buf [32]byte
    i := len(buf) - 1
    for val > 0 {
        buf[i] = alphabet[val%base]
        val /= base
        i--
    }
    return string(buf[i+1:])
}

// GenerateUniqueID возвращает уникальную строку длиной 10 символов
func GenerateUniqueID() string {
    id := atomic.AddUint64(&counter, 1)
    countStr := encodeBase62(id)
    for len(countStr) < partLen {
        countStr = "0" + countStr
    }
    if len(countStr) > partLen {
        countStr = countStr[:partLen]
    }
    return startPrefix + countStr
}


// URLShortenerInterface определяет методы сервиса сокращения URL,
// которые использует слой handler.
type URLShortenerInterface interface {
	ShortenURL(ctx context.Context, originalURL string) (string, bool, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	ShortenURLBatch(ctx context.Context, batchURL []schemasshortener.RequestBatchURLSchema) ([]schemasshortener.ResponseBatchURLSchema, error)
	GenerateID() string
	GetAllShortenerURL(ctx context.Context, userID string) ([]repo.URL, error)
}

// Shortener — глобальный сервис, который используют handler'ы.
// Инициализируется в main в зависимости от наличия рабочей БД.
var Shortener URLShortenerInterface

type shortenerService struct {
	repository repo.ShortenerRepository
}

func NewShortenerService(r repo.ShortenerRepository) URLShortenerInterface {
	return &shortenerService{repository: r}
}

func (s *shortenerService) GenerateID() string {
	b := make([]byte, 7)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (s *shortenerService) ShortenURL(ctx context.Context, originalURL string) (string, bool, error) { 
	for {
		id := s.GenerateID()
		shortCode, inserted, err := s.repository.Create(ctx, repo.URL{URL: originalURL, ShortURL: id})
		if err == nil {
			return shortCode, inserted, nil
		}
		// Коллизия short_url → пробуем сгенерировать новый ID.
		if isUniqueViolation(err) || isShortURLCollision(err) {
			continue
		}
		return "", false, err
	}
}

func (s *shortenerService) GetAllShortenerURL(ctx context.Context, userID string) ([]repo.URL, error) { 
	shortCode, err := s.repository.GetListURLByUserID(ctx, userID)
	if err == nil {
		return shortCode, nil
	}
	return []repo.URL{}, err
}


func (s *shortenerService) ShortenURLBatch(ctx context.Context, batchURL []schemasshortener.RequestBatchURLSchema) ([]schemasshortener.ResponseBatchURLSchema, error) {
	if len(batchURL) == 0 {
		return []schemasshortener.ResponseBatchURLSchema{}, nil
	}

	responses := make([]schemasshortener.ResponseBatchURLSchema, len(batchURL))
	dbBatch := make([]repo.URL, len(batchURL))
	errCh := make(chan error, len(batchURL))

	var wg sync.WaitGroup

	for i, val := range batchURL {
		wg.Add(1)
		go func(i int, val schemasshortener.RequestBatchURLSchema) {
			defer wg.Done()

			original := val.OriginalURL
			if !strings.HasPrefix(original, "http://") &&
				!strings.HasPrefix(original, "https://") {
				original = "http://" + original
			}

			id := GenerateUniqueID()
			dbBatch[i] = repo.URL{URL: original, ShortURL: id}
			
		
			shortenedURL, err := url.JoinPath(config.AppConfig.BaseURL, id)
			if err != nil {
				errCh <- err
				return
			}

			responses[i] = schemasshortener.ResponseBatchURLSchema{
				CorrelationID: val.CorrelationID,
				ShortURL:      shortenedURL,
			}
			slog.Info("Created short URL", "short_url", shortenedURL, "original", original)
		}(i, val)
	}

	wg.Wait()
	close(errCh)
	err := s.repository.CreateBatch(ctx, dbBatch)
	if err != nil {
		return nil, err
	}

	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return responses, nil
}

func (s *shortenerService) GetOriginalURL(ctx context.Context, id string) (string, error) {
	u, err := s.repository.GetByShortURL(ctx, id)
	if err != nil {
		return "", err
	}
	if u.URL == "" {
		return "", errors.New("short URL not found")
	}
	return u.URL, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isShortURLCollision(err error) bool {
	// in-memory репозиторий возвращает этот текст при коллизии
	return err != nil && err.Error() == "short_url already exists"
}

// GetService выбирает репозиторий (Postgres или in-memory) и возвращает сервис.
func GetService(pgExist bool, db interface{}) URLShortenerInterface {
	if pgExist {
		if dbConn, ok := db.(*sqlx.DB); ok {
			return NewShortenerService(repo.NewURLPostgresRepository(dbConn))
		}
	}
	mem := repo.NewRepoShortener()
	_ = mem.ExtractFromFile()
	return NewShortenerService(mem)
}
