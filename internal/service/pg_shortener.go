package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

type Url struct {
	ID       int64  `db:"id"`
	Url      string `db:"url"`
	ShortUrl string `db:"short_url"`
}

// URLShortenerInterface определяет методы для работы с короткими URL
type URLShortenerInterface interface {
	ShortenURL(ctx context.Context, originalURL string) (string, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	GenerateID() string
}

type ShortenerRepository interface {
	URLShortenerInterface
	Create(ctx context.Context, url Url) error
	GetByID(ctx context.Context, id int64) (Url, error)
	GetByUrl(ctx context.Context, url string) (Url, error)
	GetByShortUrl(ctx context.Context, shortUrl string) (Url, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]Url, error)
}

type urlPostgresRepository struct {
	db *sqlx.DB
}

func NewUrlPostgresRepository(db *sqlx.DB) ShortenerRepository {
	return &urlPostgresRepository{db: db}
}

func (r *urlPostgresRepository) Create(ctx context.Context, u Url) error {
	query := `INSERT INTO urls (url, short_url) VALUES ($1, $2) RETURNING id`
	var id int
	err := r.db.QueryRowxContext(ctx, query, u.Url, u.ShortUrl).Scan(&id)
	if err != nil {
		slog.Error(err.Error())
    }
	return err
}

func (r *urlPostgresRepository) GetByID(ctx context.Context, id int64) (Url, error) {
	var u Url
	query := `SELECT id, url, short_url FROM urls WHERE id = $1`
	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Url{}, nil
		}
		return Url{}, err
	}
	return u, nil
}

func (r *urlPostgresRepository) GetByUrl(ctx context.Context, url string) (Url, error) {
	var u Url
	query := `SELECT id, url, short_url FROM urls WHERE url = $1`
	err := r.db.GetContext(ctx, &u, query, url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Url{}, nil
		}
		return Url{}, err
	}
	return u, nil
}

func (r *urlPostgresRepository) GetByShortUrl(ctx context.Context, shortUrl string) (Url, error) {
	var u Url
	query := `SELECT id, url, short_url FROM urls WHERE short_url = $1`
	err := r.db.GetContext(ctx, &u, query, shortUrl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Url{}, nil
		}
		return Url{}, err
	}
	return u, nil
}

func (r *urlPostgresRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM urls WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *urlPostgresRepository) List(ctx context.Context) ([]Url, error) {
	var list []Url
	query := `SELECT id, url, short_url FROM urls ORDER BY id`
	err := r.db.SelectContext(ctx, &list, query)
	return list, err
}

func (r *urlPostgresRepository) GenerateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (r *urlPostgresRepository) ShortenURL(ctx context.Context, originalURL string) (string, error) {
	url, err := r.GetByUrl(ctx, originalURL)
	if err != nil {
		return "", err
	}
	if url.ShortUrl != "" {
		return url.ShortUrl, nil
	}
	// Генерируем уникальный ID
	var id string
	for {
		id = r.GenerateID()
		rowUrl, err := r.GetByShortUrl(ctx, id)
		if err != nil {
			return "", err
		}

		if rowUrl == (Url{}){
			break
		}
	}
	// Сохраняем в хранилище
	newRowUrl := Url{
		Url:      originalURL,
		ShortUrl: id,
	}
	err = r.Create(ctx, newRowUrl)
	if err != nil {
		return "", err
	}
	// us.store[id] = originalURL
	// Shortener.UpdateFile("\"" + id + "\": \"" + originalURL + "\"")
	slog.Info("Shortened URL: " + id + " -> " + originalURL)
	return newRowUrl.ShortUrl, nil 
}

func (r *urlPostgresRepository) GetOriginalURL(ctx context.Context, id string) (string, error) {
	url, err := r.GetByShortUrl(ctx, id)
	if err != nil {
		return "", err
	}
	return url.Url, nil
}
