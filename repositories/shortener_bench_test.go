package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gearwheels/go_url_shortener/internal/config"
)

func initBenchConfig(b *testing.B) {
	b.Helper()
	if config.AppConfig == nil {
		dir, err := os.MkdirTemp("", "bench")
		if err != nil {
			b.Fatal(err)
		}
		config.Init(config.InitOptions{
			ServerAddress:   "localhost:8080",
			BaseURL:         "http://localhost:8080/",
			PathStoreURL:    filepath.Join(dir, "store.txt"),
			SecretKeyForJWT: "secret",
		})
	}
}

// populatedRepo создаёт репозиторий с n записями.
func populatedRepo(b *testing.B, n int) *URLShortener {
	b.Helper()
	repo := NewRepoShortener()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		if _, _, err := repo.Create(ctx, URL{
			URL:      fmt.Sprintf("https://example.com/page%d", i),
			ShortURL: fmt.Sprintf("short%d", i),
			UserID:   fmt.Sprintf("user%d", i%10),
		}); err != nil {
			b.Fatal(err)
		}
	}
	return repo
}

func BenchmarkCreate(b *testing.B) {
	initBenchConfig(b)
	repo := NewRepoShortener()
	ctx := context.Background()
	b.ReportAllocs()
	var i int
	for b.Loop() {
		if _, _, err := repo.Create(ctx, URL{
			URL:      fmt.Sprintf("https://example.com/url%d", i),
			ShortURL: fmt.Sprintf("short%d", i),
			UserID:   "bench-user",
		}); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkGetByShortURL(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ReportAllocs()
	var i int
	for b.Loop() {
		if _, err := repo.GetByShortURL(ctx, fmt.Sprintf("short%d", i%1000)); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkGetByURL(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ReportAllocs()
	var i int
	for b.Loop() {
		if _, err := repo.GetByURL(ctx, fmt.Sprintf("https://example.com/page%d", i%1000)); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkList(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := repo.List(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetListURLByUserID(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		if _, err := repo.GetListURLByUserID(ctx, "user5"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBatch(b *testing.B) {
	initBenchConfig(b)
	ctx := context.Background()
	batch := make([]URL, 50)
	b.ReportAllocs()
	var i int
	for b.Loop() {
		r := NewRepoShortener()
		for j := range batch {
			batch[j] = URL{
				URL:      fmt.Sprintf("https://example.com/batch%d-%d", i, j),
				ShortURL: fmt.Sprintf("b%d-%d", i, j),
				UserID:   "bench-user",
			}
		}
		if err := r.CreateBatch(ctx, batch); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkUpdateIsDelete(b *testing.B) {
	initBenchConfig(b)
	ctx := context.Background()
	repo := populatedRepo(b, 100)
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = fmt.Sprintf("short%d", i)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := repo.UpdateIsDelete(ctx, "user0", ids); err != nil {
			b.Fatal(err)
		}
	}
}
