package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

type URL struct {
	ID          int64  `db:"id"`
	URL         string `db:"url"`
	ShortURL    string `db:"short_url"`
	UserID      string `db:"user_id"`
	DeletedFlag bool   `db:"is_deleted"`
}

type ShortenerRepository interface {
	CreateBatch(ctx context.Context, URLBatch []URL) (err error)
	Create(ctx context.Context, url URL) (shortCode string, inserted bool, err error)
	GetByID(ctx context.Context, id int64) (URL, error)
	GetByURL(ctx context.Context, url string) (URL, error)
	GetByShortURL(ctx context.Context, shortURL string) (URL, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]URL, error)
	GetListURLByUserID(ctx context.Context, userID string) ([]URL, error)
	UpdateIsDelete(ctx context.Context, userID string, shortIDs []string) error
	DeleteByShortURL(ctx context.Context, userID string, shortURL string) error
}

type URLPostgresRepository struct {
	db *sqlx.DB
}

// NewURLPostgresRepository возвращает конкретную реализацию репозитория.
// Репозиторий содержит только методы работы с данными.
func NewURLPostgresRepository(db *sqlx.DB) *URLPostgresRepository {
	return &URLPostgresRepository{db: db}
}

func (r *URLPostgresRepository) Create(ctx context.Context, u URL) (shortCode string, inserted bool, err error) {
	query := `
	INSERT INTO urls (url, short_url, user_id)
	VALUES ($1, $2, $3)
	ON CONFLICT (url) DO UPDATE
		SET url = EXCLUDED.url
	RETURNING short_url, (xmax = 0) AS inserted
	`
	err = r.db.QueryRowContext(ctx, query, u.URL, u.ShortURL, u.UserID).Scan(&shortCode, &inserted)
	if err != nil {
		return "", false, err
	}
	return shortCode, inserted, nil
}

func (r *URLPostgresRepository) GetListURLByUserID(ctx context.Context, userID string) ([]URL, error) {
	var list []URL
	query := `SELECT id, url, short_url, user_id FROM urls WHERE user_id = $1 ORDER BY id`
	err := r.db.SelectContext(ctx, &list, query, userID)
	return list, err
}

func (r *URLPostgresRepository) GetTx() (*sql.Tx, error) {
	return r.db.Begin()
}

func (r *URLPostgresRepository) CreateBatch(ctx context.Context, URLBatch []URL) (err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := `
	INSERT INTO urls (url, short_url, user_id)
	VALUES ($1, $2, $3)
	ON CONFLICT (url) DO UPDATE
		SET url = EXCLUDED.url
	RETURNING short_url, (xmax = 0) AS inserted
	`
	for _, u := range URLBatch {
		_, err = tx.ExecContext(ctx, query, u.URL, u.ShortURL, u.UserID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *URLPostgresRepository) GetByID(ctx context.Context, id int64) (URL, error) {
	var u URL
	query := `SELECT id, url, short_url FROM urls WHERE id = $1`
	err := r.db.GetContext(ctx, &u, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL{}, nil
		}
		return URL{}, err
	}
	return u, nil
}

func (r *URLPostgresRepository) GetByURL(ctx context.Context, url string) (URL, error) {
	var u URL
	query := `SELECT id, url, short_url FROM urls WHERE url = $1`
	err := r.db.GetContext(ctx, &u, query, url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL{}, nil
		}
		return URL{}, err
	}
	return u, nil
}

func (r *URLPostgresRepository) GetByShortURL(ctx context.Context, shortURL string) (URL, error) {
	var u URL
	query := `SELECT id, url, short_url, is_deleted FROM urls WHERE short_url = $1`
	err := r.db.GetContext(ctx, &u, query, shortURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return URL{}, nil
		}
		return URL{}, err
	}
	return u, nil
}

func (r *URLPostgresRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM urls WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *URLPostgresRepository) DeleteByShortURL(ctx context.Context, userID string, shortURL string) error {
	query := `DELETE FROM urls WHERE user_id = $1 AND short_url = $2`
	_, err := r.db.ExecContext(ctx, query, userID, shortURL)
	return err
}

func (r *URLPostgresRepository) DeleteByShortURLInTx(ctx context.Context, tx *sql.Tx, userID string, shortURL string) error {
	query := `DELETE FROM urls WHERE user_id = $1 AND short_url = $2`
	_, err := tx.ExecContext(ctx, query, userID, shortURL)
	return err
}

func (r *URLPostgresRepository) List(ctx context.Context) ([]URL, error) {
	var list []URL
	query := `SELECT id, url, short_url, user_id FROM urls ORDER BY id`
	err := r.db.SelectContext(ctx, &list, query)
	return list, err
}

// Update устанавливает флаг is_deleted для набора short_url конкретного пользователя.
func (r *URLPostgresRepository) UpdateIsDelete(ctx context.Context, userID string, shortIDs []string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE urls
		SET is_deleted = TRUE
		WHERE user_id = $1
		  AND short_url = $2
	`

	for _, shortID := range shortIDs {
		_, err = tx.ExecContext(ctx, query, userID, shortID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
