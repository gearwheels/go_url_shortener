package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

type URL struct {
	ID       int64  `db:"id"`
	URL      string `db:"url"`
	ShortURL string `db:"short_url"`
}

// URLShortenerInterface определяет методы для работы с короткими URL
type URLShortenerInterface interface {
	ShortenURL(ctx context.Context, originalURL string) (string, bool, error)
	GetOriginalURL(ctx context.Context, id string) (string, error)
	GenerateID() string
}

type ShortenerRepository interface {
	URLShortenerInterface
	Create(ctx context.Context, url URL) (shortCode string, inserted bool, err error)
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

func (r *urlPostgresRepository) Create(ctx context.Context, u URL) (shortCode string, inserted bool, err error) {
	query := `
	INSERT INTO urls (url, short_url)
	VALUES ($1, $2)
	ON CONFLICT (url) DO UPDATE
		SET url = EXCLUDED.url
	RETURNING short_url, (xmax = 0) AS inserted
	`
	err = r.db.QueryRowContext(ctx, query, u.URL, u.ShortURL).Scan(&shortCode, &inserted)
	if err != nil {
		return "", false, err
	}
	return shortCode, inserted, nil
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

func (r *urlPostgresRepository) ShortenURL(ctx context.Context, originalURL string) (string, bool, error) {
	var shortCode string
	var inserted bool
	var err error
	for {
		id := r.GenerateID()
		shortCode, inserted, err = r.Create(ctx, URL{URL: originalURL, ShortURL: id})
		if err == nil {
			break
		}
		// Повторяем только при конфликте по short_code (коллизия генерации ID)
		if !isConflictOnShortCode(err) {
			return "", false, err
		}
	}
	slog.Info("Shortened URL: " + shortCode + " -> " + originalURL)
	return shortCode, inserted, nil
}

// isConflictOnShortCode проверяет, что ошибка — нарушение уникальности (в т.ч. по short_code).
func isConflictOnShortCode(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" // unique_violation
	}
	return false
}

func (r *urlPostgresRepository) GetOriginalURL(ctx context.Context, id string) (string, error) {
	u, err := r.GetByShortURL(ctx, id)
	if err != nil {
		return "", err
	}
	if u.URL == "" {
		return "", sql.ErrNoRows
	}
	return u.URL, nil
}
