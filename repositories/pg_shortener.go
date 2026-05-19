// Package repository реализует уровень доступа к данным для сервиса сокращения URL.
// Содержит две реализации ShortenerRepository:
//   - URLShortener — in-memory хранилище с персистентностью через файл.
//   - URLPostgresRepository — PostgreSQL хранилище на основе sqlx.
package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

// URL хранит данные одной записи сокращённого адреса.
type URL struct {
	// ID — суррогатный ключ (автоинкремент в БД, счётчик в памяти).
	ID int64 `db:"id"`
	// URL — оригинальный длинный адрес.
	URL string `db:"url"`
	// ShortURL — короткий идентификатор (случайная строка).
	ShortURL string `db:"short_url"`
	// UserID — идентификатор пользователя-владельца.
	UserID string `db:"user_id"`
	// DeletedFlag — true, если ссылка помечена на удаление (soft delete).
	DeletedFlag bool `db:"is_deleted"`
}

// ShortenerRepository определяет контракт хранилища коротких URL.
// Реализуется как in-memory (URLShortener), так и Postgres (URLPostgresRepository).
type ShortenerRepository interface {
	// Create сохраняет новую запись и возвращает shortCode.
	// Если URL уже существует, возвращает существующий shortCode и inserted=false.
	Create(ctx context.Context, url URL) (shortCode string, inserted bool, err error)
	// CreateBatch сохраняет несколько записей в одной транзакции.
	CreateBatch(ctx context.Context, URLBatch []URL) (err error)
	// GetByID ищет запись по числовому ID.
	GetByID(ctx context.Context, id int64) (URL, error)
	// GetByURL ищет запись по оригинальному URL.
	GetByURL(ctx context.Context, url string) (URL, error)
	// GetByShortURL ищет запись по короткому идентификатору.
	GetByShortURL(ctx context.Context, shortURL string) (URL, error)
	// Delete удаляет запись по числовому ID.
	Delete(ctx context.Context, id int64) error
	// List возвращает все записи, отсортированные по ID.
	List(ctx context.Context) ([]URL, error)
	// GetListURLByUserID возвращает все ссылки указанного пользователя.
	GetListURLByUserID(ctx context.Context, userID string) ([]URL, error)
	// UpdateIsDelete помечает список коротких URL пользователя как удалённые.
	UpdateIsDelete(ctx context.Context, userID string, shortIDs []string) error
	// DeleteByShortURL физически удаляет запись по short_url и userID.
	DeleteByShortURL(ctx context.Context, userID string, shortURL string) error
}

// URLPostgresRepository — реализация ShortenerRepository поверх PostgreSQL.
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

// GetTx открывает новую транзакцию базы данных.
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

// DeleteByShortURLInTx физически удаляет запись в рамках существующей транзакции tx.
// Используется воркером фонового удаления для батчинга нескольких операций.
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
