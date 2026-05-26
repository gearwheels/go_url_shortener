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
		config.Init("localhost:8080", "http://localhost:8080/", filepath.Join(dir, "store.txt"),
			"", "secret", "", "")
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
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, _, err := repo.Create(ctx, URL{
			URL:      fmt.Sprintf("https://example.com/url%d", i),
			ShortURL: fmt.Sprintf("short%d", i),
			UserID:   "bench-user",
		}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetByShortURL(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := repo.GetByShortURL(ctx, fmt.Sprintf("short%d", i%1000)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetByURL(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := repo.GetByURL(ctx, fmt.Sprintf("https://example.com/page%d", i%1000)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkList(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := repo.List(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetListURLByUserID(b *testing.B) {
	initBenchConfig(b)
	repo := populatedRepo(b, 1000)
	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := repo.GetListURLByUserID(ctx, "user5"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBatch(b *testing.B) {
	initBenchConfig(b)
	ctx := context.Background()
	batch := make([]URL, 50)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		r := NewRepoShortener()
		for j := range batch {
			batch[j] = URL{
				URL:      fmt.Sprintf("https://example.com/batch%d-%d", i, j),
				ShortURL: fmt.Sprintf("b%d-%d", i, j),
				UserID:   "bench-user",
			}
		}
		b.StartTimer()
		_ = r.CreateBatch(ctx, batch)
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
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = repo.UpdateIsDelete(ctx, "user0", ids)
	}
}
