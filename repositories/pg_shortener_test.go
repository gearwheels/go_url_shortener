package repository

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

	t.Run("success insert", func(t *testing.T) {
		mock.ExpectQuery(`INSERT INTO urls`).
			WithArgs("https://example.com", "abc123", "").
			WillReturnRows(sqlmock.NewRows([]string{"short_url", "inserted"}).AddRow("abc123", true))

		shortCode, inserted, err := repo.Create(ctx, URL{
			URL:      "https://example.com",
			ShortURL: "abc123",
		})
		if err != nil {
			t.Errorf("Create: %v", err)
		}
		if shortCode != "abc123" || !inserted {
			t.Errorf("Create: shortCode=%q inserted=%v", shortCode, inserted)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("expectations: %v", err)
		}
	})

	t.Run("success conflict returns existing", func(t *testing.T) {
		mock.ExpectQuery(`INSERT INTO urls`).
			WithArgs("https://example.com", "newid", "").
			WillReturnRows(sqlmock.NewRows([]string{"short_url", "inserted"}).AddRow("existing123", false))

		shortCode, inserted, err := repo.Create(ctx, URL{URL: "https://example.com", ShortURL: "newid"})
		if err != nil {
			t.Errorf("Create: %v", err)
		}
		if shortCode != "existing123" || inserted {
			t.Errorf("Create: shortCode=%q inserted=%v", shortCode, inserted)
		}
	})

	t.Run("error", func(t *testing.T) {
		mock.ExpectQuery(`INSERT INTO urls`).
			WithArgs("https://bad.com", "x", "").
			WillReturnError(sql.ErrConnDone)

		_, _, err := repo.Create(ctx, URL{URL: "https://bad.com", ShortURL: "x"})
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
		mock.ExpectQuery(`SELECT id, url, short_url, is_deleted FROM urls WHERE short_url = \$1`).
			WithArgs(shortURL).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url", "is_deleted"}).
				AddRow(1, "https://example.com", shortURL, false))

		u, err := repo.GetByShortURL(ctx, shortURL)
		if err != nil {
			t.Errorf("GetByShortURL: %v", err)
		}
		if u.ShortURL != shortURL || u.URL != "https://example.com" {
			t.Errorf("got %+v", u)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock.ExpectQuery(`SELECT id, url, short_url, is_deleted FROM urls WHERE short_url = \$1`).
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
		mock.ExpectQuery(`SELECT id, url, short_url, user_id FROM urls ORDER BY id`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url", "user_id"}).
				AddRow(1, "https://a.com", "id1", "").
				AddRow(2, "https://b.com", "id2", ""))

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
		mock.ExpectQuery(`SELECT id, url, short_url, user_id FROM urls ORDER BY id`).
			WillReturnRows(sqlmock.NewRows([]string{"id", "url", "short_url", "user_id"}))

		list, err := repo.List(ctx)
		if err != nil {
			t.Errorf("List: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("expected 0 rows, got %d", len(list))
		}
	})
}

func TestURLPostgresRepository_CreateBatch(t *testing.T) {
	sqlxDB, mock := newTestDB(t)
	defer sqlxDB.Close()

	repo := NewURLPostgresRepository(sqlxDB)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		batch := []URL{
			{URL: "https://example.com/a", ShortURL: "shortA"},
			{URL: "https://example.com/b", ShortURL: "shortB"},
		}

		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO urls`).
			WithArgs("https://example.com/a", "shortA", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`INSERT INTO urls`).
			WithArgs("https://example.com/b", "shortB", "").
			WillReturnResult(sqlmock.NewResult(2, 1))
		mock.ExpectCommit()

		err := repo.CreateBatch(ctx, batch)
		if err != nil {
			t.Errorf("CreateBatch: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("expectations: %v", err)
		}
	})

	t.Run("empty batch", func(t *testing.T) {
		mock.ExpectBegin()
		mock.ExpectCommit()

		err := repo.CreateBatch(ctx, []URL{})
		if err != nil {
			t.Errorf("CreateBatch(empty): %v", err)
		}
	})

	t.Run("rollback on error", func(t *testing.T) {
		batch := []URL{
			{URL: "https://example.com/x", ShortURL: "shortX"},
		}
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO urls`).
			WithArgs("https://example.com/x", "shortX", "").
			WillReturnError(sql.ErrConnDone)
		mock.ExpectRollback()

		err := repo.CreateBatch(ctx, batch)
		if err == nil {
			t.Error("CreateBatch expected error")
		}
		if err != sql.ErrConnDone {
			t.Errorf("CreateBatch: expected %v, got %v", sql.ErrConnDone, err)
		}
	})
}
