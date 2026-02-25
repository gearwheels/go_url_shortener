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
	config.Init("localhost:8888", "http://localhost:8000/", path, "postgres://shortener:shortener@localhost:5432/shortener")
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
