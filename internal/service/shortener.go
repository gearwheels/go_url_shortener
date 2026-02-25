package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"

	repo "github.com/gearwheels/go_url_shortener/repositories"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

// URLShortenerInterface определяет методы сервиса сокращения URL,
// которые использует слой handler.
type URLShortenerInterface interface {
	ShortenURL(ctx context.Context, originalURL string) (string, bool, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	GenerateID() string
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
	b := make([]byte, 6)
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

