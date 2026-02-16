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

type URL struct {
	ID       int64  `db:"id"`
	URL      string `db:"url"`
	ShortURL string `db:"short_url"`
}

// URLShortenerInterface определяет методы для работы с короткими URL
type URLShortenerInterface interface {
	ShortenURL(ctx context.Context, originalURL string) (string, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	GenerateID() string
}

type ShortenerRepository interface {
	URLShortenerInterface
	Create(ctx context.Context, url URL ) error
	GetByID(ctx context.Context, id int64) (URL , error)
	GetByURL(ctx context.Context, url string) (URL , error)
	GetByShortURL (ctx context.Context, shortURL string) (URL , error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]URL , error)
}

type urlPostgresRepository struct {
	db *sqlx.DB
}

func NewURLPostgresRepository(db *sqlx.DB) ShortenerRepository {
	return &urlPostgresRepository{db: db}
}

func (r *urlPostgresRepository) Create(ctx context.Context, u URL ) error {
	query := `INSERT INTO urls (url, short_url) VALUES ($1, $2) RETURNING id`
	var id int
	err := r.db.QueryRowxContext(ctx, query, u.URL , u.ShortURL ).Scan(&id)
	if err != nil {
		slog.Error(err.Error())
	}
	return err
}

func (r *urlPostgresRepository) GetByID(ctx context.Context, id int64) (URL , error) {
	var u URL 
	query := `SELECT id, url, short_url FROM urls WHERE id = $1`
	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL {}, nil
		}
		return URL {}, err
	}
	return u, nil
}

func (r *urlPostgresRepository) GetByURL(ctx context.Context, url string) (URL , error) {
	var u URL 
	query := `SELECT id, url, short_url FROM urls WHERE url = $1`
	err := r.db.GetContext(ctx, &u, query, url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL {}, nil
		}
		return URL {}, err
	}
	return u, nil
}

func (r *urlPostgresRepository) GetByShortURL (ctx context.Context, shortURL string) (URL , error) {
	var u URL 
	query := `SELECT id, url, short_url FROM urls WHERE short_url = $1`
	err := r.db.GetContext(ctx, &u, query, shortURL )
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL {}, nil
		}
		return URL {}, err
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

func (r *urlPostgresRepository) List(ctx context.Context) ([]URL , error) {
	var list []URL 
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
	url, err := r.GetByURL(ctx, originalURL)
	if err != nil {
		return "", err
	}
	if url.ShortURL != "" {
		return url.ShortURL , nil
	}
	// Генерируем уникальный ID
	var id string
	for {
		id = r.GenerateID()
		rowURL , err := r.GetByShortURL (ctx, id)
		if err != nil {
			return "", err
		}

		if rowURL == (URL {}) {
			break
		}
	}
	// Сохраняем в хранилище
	newRowURL := URL {
		URL :      originalURL,
		ShortURL : id,
	}
	err = r.Create(ctx, newRowURL )
	if err != nil {
		return "", err
	}
	// us.store[id] = originalURL
	// Shortener.UpdateFile("\"" + id + "\": \"" + originalURL + "\"")
	slog.Info("Shortened URL: " + id + " -> " + originalURL)
	return newRowURL .ShortURL , nil
}

func (r *urlPostgresRepository) GetOriginalURL(ctx context.Context, id string) (string, error) {
	url, err := r.GetByShortURL (ctx, id)
	if err != nil {
		return "", err
	}
	return url.URL , nil
}
