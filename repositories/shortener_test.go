package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "shortener_svc_test")
	if err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "store_url.txt")
	config.Init("localhost:8888", "http://localhost:8000/", path, "postgres://shortener:shortener@localhost:5432/shortener", "test-secret")
	os.Exit(m.Run())
}

func TestInMemoryRepository_CRUD(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	// create new
	short, inserted, err := repo.Create(ctx, URL{URL: "https://example.com", ShortURL: "abc"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if short != "abc" || !inserted {
		t.Fatalf("Create: short=%q inserted=%v", short, inserted)
	}

	// duplicate url
	short2, inserted2, err := repo.Create(ctx, URL{URL: "https://example.com", ShortURL: "zzz"})
	if err != nil {
		t.Fatalf("Create duplicate url: %v", err)
	}
	if short2 != "abc" || inserted2 {
		t.Fatalf("Create duplicate url: short=%q inserted=%v", short2, inserted2)
	}

	// get by url
	u, err := repo.GetByURL(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if u.ShortURL != "abc" {
		t.Fatalf("GetByURL: got %+v", u)
	}

	// list
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1, got %d", len(list))
	}

	// delete
	if err := repo.Delete(ctx, list[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if after, _ := repo.List(ctx); len(after) != 0 {
		t.Fatalf("List after delete: expected 0, got %d", len(after))
	}
}

func TestInMemoryRepository_CreateBatch(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	batch := []URL{
		{URL: "https://example.com/1", ShortURL: "id1"},
		{URL: "https://example.com/2", ShortURL: "id2"},
		{URL: "https://example.com/3", ShortURL: "id3"},
	}

	err := repo.CreateBatch(ctx, batch)
	if err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List: expected 3, got %d", len(list))
	}

	for _, u := range list {
		byShort, err := repo.GetByShortURL(ctx, u.ShortURL)
		if err != nil {
			t.Fatalf("GetByShortURL(%s): %v", u.ShortURL, err)
		}
		if byShort.URL != u.URL {
			t.Errorf("GetByShortURL(%s): expected URL %s, got %s", u.ShortURL, u.URL, byShort.URL)
		}
		byURL, err := repo.GetByURL(ctx, u.URL)
		if err != nil {
			t.Fatalf("GetByURL(%s): %v", u.URL, err)
		}
		if byURL.ShortURL != u.ShortURL {
			t.Errorf("GetByURL(%s): expected ShortURL %s, got %s", u.URL, u.ShortURL, byURL.ShortURL)
		}
	}
}

func TestInMemoryRepository_CreateBatch_Empty(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	err := repo.CreateBatch(ctx, nil)
	if err != nil {
		t.Fatalf("CreateBatch(nil): %v", err)
	}
	err = repo.CreateBatch(ctx, []URL{})
	if err != nil {
		t.Fatalf("CreateBatch(empty): %v", err)
	}

	list, _ := repo.List(ctx)
	if len(list) != 0 {
		t.Errorf("expected 0 records, got %d", len(list))
	}
}
