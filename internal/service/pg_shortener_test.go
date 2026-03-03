package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
)

func newTestDB(t *testing.T) (*sqlx.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	sqlxDB := sqlx.NewDb(db, "pgx")
	return sqlxDB, mock
}

func TestURLPostgresRepository_Create(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(`INSERT INTO urls \(url, short_url\) VALUES \(\$1, \$2\) RETURNING id`).
			WithArgs("https://example.com", "abc123").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		err := repo.Create(ctx, URL{
			URL:      "https://example.com",
			ShortURL: "abc123",
		})
		if err != nil {
			t.Errorf("Create: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("expectations: %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectQuery(`INSERT INTO urls \(url, short_url\) VALUES \(\$1, \$2\) RETURNING id`).
			WithArgs("https://bad.com", "x").
			WillReturnError(sql.ErrConnDone)

		err := repo.Create(ctx, URL{URL: "https://bad.com", ShortURL: "x"})
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestURLPostgresRepository_GetByID(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE id = \$1`).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, "https://example.com", "abc123"))

		u, err := repo.GetByID(ctx, 1)
		if err != nil {
			t.Errorf("GetByID: %v", err)
		}
		if u.ID != 1 || u.URL != "https://example.com" || u.ShortURL != "abc123" {
			t.Errorf("got %+v", u)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE id = \$1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		u, err := repo.GetByID(ctx, 999)
		if err != nil {
			t.Errorf("GetByID (not found) should return nil error: %v", err)
		}
		if u.URL != "" || u.ShortURL != "" {
			t.Errorf("expected zero value, got %+v", u)
		}
	})
}

func TestURLPostgresRepository_GetByURL(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		url := "https://example.com"
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE url = \$1`).
			WithArgs(url).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, url, "abc123"))

		u, err := repo.GetByURL(ctx, url)
		if err != nil {
			t.Errorf("GetByURL: %v", err)
		}
		if u.URL != url || u.ShortURL != "abc123" {
			t.Errorf("got %+v", u)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE url = \$1`).
			WithArgs("https://missing.com").
			WillReturnError(sql.ErrNoRows)

		u, err := repo.GetByURL(ctx, "https://missing.com")
		if err != nil {
			t.Errorf("GetByURL (not found) should return nil error: %v", err)
		}
		if u.URL != "" {
			t.Errorf("expected zero value, got %+v", u)
		}
	})
}

func TestURLPostgresRepository_GetByShortURL(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		shortURL := "abc123"
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE short_url = \$1`).
			WithArgs(shortURL).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, "https://example.com", shortURL))

		u, err := repo.GetByShortURL(ctx, shortURL)
		if err != nil {
			t.Errorf("GetByShortURL: %v", err)
		}
		if u.ShortURL != shortURL || u.URL != "https://example.com" {
			t.Errorf("got %+v", u)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE short_url = \$1`).
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		u, err := repo.GetByShortURL(ctx, "nonexistent")
		if err != nil {
			t.Errorf("GetByShortURL (not found) should return nil error: %v", err)
		}
		if u.ShortURL != "" {
			t.Errorf("expected zero value, got %+v", u)
		}
	})
}

func TestURLPostgresRepository_Delete(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM urls WHERE id = \$1`).
			WithArgs(int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := repo.Delete(ctx, 1)
		if err != nil {
			t.Errorf("Delete: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectExec(`DELETE FROM urls WHERE id = \$1`).
			WithArgs(int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.Delete(ctx, 999)
		if err != sql.ErrNoRows {
			t.Errorf("expected sql.ErrNoRows, got %v", err)
		}
	})
}

func TestURLPostgresRepository_List(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls ORDER BY id`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, "https://a.com", "id1").
				AddRow(2, "https://b.com", "id2"))

		list, err := repo.List(ctx)
		if err != nil {
			t.Errorf("List: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 rows, got %d", len(list))
		}
		if list[0].URL != "https://a.com" || list[0].ShortURL != "id1" {
			t.Errorf("list[0] = %+v", list[0])
		}
		if list[1].URL != "https://b.com" || list[1].ShortURL != "id2" {
			t.Errorf("list[1] = %+v", list[1])
		}
	})

	t.Run("empty", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls ORDER BY id`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}))

		list, err := repo.List(ctx)
		if err != nil {
			t.Errorf("List: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected 0 rows, got %d", len(list))
		}
	})
}

func TestURLPostgresRepository_ShortenURL(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("new url", func(t *testing.T) {
		originalURL := "https://example.com/new"
		// GetByURL — не найден
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE url = \$1`).
			WithArgs(originalURL).
			WillReturnError(sql.ErrNoRows)
		// GetByShortURL в цикле — первый вызов не найден (уникальный short id)
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE short_url = \$1`).
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)
		// Create
		mock.ExpectQuery(`INSERT INTO urls \(url, short_url\) VALUES \(\$1, \$2\) RETURNING id`).
			WithArgs(originalURL, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

		short, err := repo.ShortenURL(ctx, originalURL)
		if err != nil {
			t.Errorf("ShortenURL: %v", err)
		}
		if short == "" {
			t.Error("expected non-empty short URL")
		}
	})

	t.Run("duplicate url returns existing short", func(t *testing.T) {
		originalURL := "https://example.com/dup"
		existingShort := "existing123"
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE url = \$1`).
			WithArgs(originalURL).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, originalURL, existingShort))

		short, err := repo.ShortenURL(ctx, originalURL)
		if err != nil {
			t.Errorf("ShortenURL: %v", err)
		}
		if short != existingShort {
			t.Errorf("expected %q, got %q", existingShort, short)
		}
	})
}

func TestURLPostgresRepository_GetOriginalURL(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		shortID := "abc123"
		original := "https://example.com"
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE short_url = \$1`).
			WithArgs(shortID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url"}).
				AddRow(1, original, shortID))

		got, err := repo.GetOriginalURL(ctx, shortID)
		if err != nil {
			t.Errorf("GetOriginalURL: %v", err)
		}
		if got != original {
			t.Errorf("expected %q, got %q", original, got)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url FROM urls WHERE short_url = \$1`).
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		got, err := repo.GetOriginalURL(ctx, "nonexistent")
		if err != nil {
			t.Errorf("GetOriginalURL (not found) returned error: %v", err)
		}
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestURLPostgresRepository_GenerateID(t *testing.T) {
	sqlxDB, _ := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)

	id := repo.GenerateID()
	if id == "" {
		t.Error("GenerateID returned empty string")
	}
	// RawURLEncoding produces [A-Za-z0-9_-]
	for _, c := range id {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		t.Errorf("GenerateID returned invalid character %q in %q", c, id)
	}
	// Uniqueness in a small sample
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := repo.GenerateID()
		if seen[id] {
			t.Errorf("duplicate ID %q", id)
		}
		seen[id] = true
	}
}
