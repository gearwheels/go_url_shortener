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
	config.Init(config.InitOptions{
		ServerAddress:   "localhost:8888",
		BaseURL:         "http://localhost:8000/",
		PathStoreURL:    path,
		DatabaseDsn:     "postgres://shortener:shortener@localhost:5432/shortener",
		SecretKeyForJWT: "test-secret",
	})
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
	if delErr := repo.Delete(ctx, list[0].ID); delErr != nil {
		t.Fatalf("Delete: %v", delErr)
	}
	after, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(after) != 0 {
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

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 records, got %d", len(list))
	}
}

func TestInMemoryRepository_GetByID(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	_, inserted, err := repo.Create(ctx, URL{URL: "https://example.com", ShortURL: "abc"})
	if err != nil || !inserted {
		t.Fatalf("Create: %v, inserted=%v", err, inserted)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 item")
	}

	u, err := repo.GetByID(ctx, list[0].ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if u.URL != "https://example.com" {
		t.Errorf("GetByID: expected example.com, got %q", u.URL)
	}

	missing, err := repo.GetByID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetByID missing: %v", err)
	}
	if missing.URL != "" {
		t.Errorf("GetByID missing: expected empty URL, got %q", missing.URL)
	}
}

func TestInMemoryRepository_DeleteByShortURL(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	_, _, err := repo.Create(ctx, URL{URL: "https://example.com/del", ShortURL: "del1", UserID: "u1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = repo.DeleteByShortURL(ctx, "u1", "del1")
	if err != nil {
		t.Fatalf("DeleteByShortURL: %v", err)
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty after delete, got %d", len(list))
	}

	err = repo.DeleteByShortURL(ctx, "u1", "notexist")
	if err == nil {
		t.Error("expected error for missing short URL")
	}
}

func TestInMemoryRepository_UpdateIsDelete(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	if _, _, err := repo.Create(ctx, URL{URL: "https://a.com", ShortURL: "s1", UserID: "u1"}); err != nil {
		t.Fatalf("Create s1: %v", err)
	}
	if _, _, err := repo.Create(ctx, URL{URL: "https://b.com", ShortURL: "s2", UserID: "u1"}); err != nil {
		t.Fatalf("Create s2: %v", err)
	}
	if _, _, err := repo.Create(ctx, URL{URL: "https://c.com", ShortURL: "s3", UserID: "u2"}); err != nil {
		t.Fatalf("Create s3: %v", err)
	}

	err := repo.UpdateIsDelete(ctx, "u1", []string{"s1"})
	if err != nil {
		t.Fatalf("UpdateIsDelete: %v", err)
	}

	u, err := repo.GetByShortURL(ctx, "s1")
	if err != nil {
		t.Fatalf("GetByShortURL s1: %v", err)
	}
	if !u.DeletedFlag {
		t.Error("s1 should be marked deleted")
	}
	u2, err := repo.GetByShortURL(ctx, "s2")
	if err != nil {
		t.Fatalf("GetByShortURL s2: %v", err)
	}
	if u2.DeletedFlag {
		t.Error("s2 should NOT be marked deleted")
	}
	u3, err := repo.GetByShortURL(ctx, "s3")
	if err != nil {
		t.Fatalf("GetByShortURL s3: %v", err)
	}
	if u3.DeletedFlag {
		t.Error("s3 (different user) should NOT be marked deleted")
	}

	err = repo.UpdateIsDelete(ctx, "u1", nil)
	if err != nil {
		t.Fatalf("UpdateIsDelete(nil): %v", err)
	}
}

func TestInMemoryRepository_GetListURLByUserID(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	if _, _, err := repo.Create(ctx, URL{URL: "https://a.com", ShortURL: "a1", UserID: "alice"}); err != nil {
		t.Fatalf("Create a1: %v", err)
	}
	if _, _, err := repo.Create(ctx, URL{URL: "https://b.com", ShortURL: "b1", UserID: "alice"}); err != nil {
		t.Fatalf("Create b1: %v", err)
	}
	if _, _, err := repo.Create(ctx, URL{URL: "https://c.com", ShortURL: "c1", UserID: "bob"}); err != nil {
		t.Fatalf("Create c1: %v", err)
	}

	aliceList, err := repo.GetListURLByUserID(ctx, "alice")
	if err != nil {
		t.Fatalf("GetListURLByUserID: %v", err)
	}
	if len(aliceList) != 2 {
		t.Errorf("expected 2 for alice, got %d", len(aliceList))
	}

	bobList, err := repo.GetListURLByUserID(ctx, "bob")
	if err != nil {
		t.Fatalf("GetListURLByUserID bob: %v", err)
	}
	if len(bobList) != 1 {
		t.Errorf("expected 1 for bob, got %d", len(bobList))
	}

	noneList, err := repo.GetListURLByUserID(ctx, "nobody")
	if err != nil {
		t.Fatalf("GetListURLByUserID nobody: %v", err)
	}
	if len(noneList) != 0 {
		t.Errorf("expected 0 for nobody, got %d", len(noneList))
	}
}

func TestInMemoryRepository_HelperMethods(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	if _, _, err := repo.Create(ctx, URL{URL: "https://x.com", ShortURL: "x1"}); err != nil {
		t.Fatalf("Create x1: %v", err)
	}

	repo.RLockMu()
	n := repo.GetLenStore()
	repo.RUnlockMu()
	if n != 1 {
		t.Errorf("GetLenStore: expected 1, got %d", n)
	}

	jsonBytes := repo.PrintStore()
	if len(jsonBytes) == 0 {
		t.Error("PrintStore: expected non-empty JSON")
	}

	repo.FreeStore()
	if repo.GetLenStore() != 0 {
		t.Error("FreeStore: expected 0 after free")
	}
}

func TestInMemoryRepository_ExtractFromFile(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	if _, _, err := repo.Create(ctx, URL{URL: "https://persist.com", ShortURL: "p1"}); err != nil {
		t.Fatalf("Create p1: %v", err)
	}
	if err := repo.UpdateFile(`"p1":"https://persist.com"`); err != nil {
		t.Fatalf("UpdateFile: %v", err)
	}

	repo2 := NewRepoShortener()
	if err := repo2.ExtractFromFile(); err != nil {
		t.Fatalf("ExtractFromFile: %v", err)
	}

	u, err := repo2.GetByShortURL(ctx, "p1")
	if err != nil {
		t.Fatalf("GetByShortURL after extract: %v", err)
	}
	if u.URL != "https://persist.com" {
		t.Errorf("expected persist.com, got %q", u.URL)
	}
}

func TestInMemoryRepository_Create_ShortURLCollision(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	_, _, err := repo.Create(ctx, URL{URL: "https://first.com", ShortURL: "same"})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	_, _, err = repo.Create(ctx, URL{URL: "https://second.com", ShortURL: "same"})
	if err == nil {
		t.Error("expected collision error for duplicate ShortURL with different URL")
	}
}

func TestInMemoryRepository_GetByURL_Missing(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	u, err := repo.GetByURL(ctx, "https://nothere.com")
	if err != nil {
		t.Fatalf("GetByURL missing: %v", err)
	}
	if u.URL != "" {
		t.Errorf("expected empty URL for missing key, got %q", u.URL)
	}
}

func TestInMemoryRepository_GetByShortURL_Missing(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	u, err := repo.GetByShortURL(ctx, "notexist")
	if err != nil {
		t.Fatalf("GetByShortURL missing: %v", err)
	}
	if u.URL != "" {
		t.Errorf("expected empty URL for missing short, got %q", u.URL)
	}
}

func TestInMemoryRepository_Delete_Missing(t *testing.T) {
	repo := NewRepoShortener()
	ctx := context.Background()

	err := repo.Delete(ctx, 99999)
	if err == nil {
		t.Error("expected error when deleting non-existent ID")
	}
}
