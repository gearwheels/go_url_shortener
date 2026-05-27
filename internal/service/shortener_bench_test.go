package service

import (
	"context"
	"fmt"
	"testing"

	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

func newBenchService() URLShortenerInterface {
	return NewShortenerService(repo.NewRepoShortener())
}

func BenchmarkGenerateID(b *testing.B) {
	svc := newBenchService().(*shortenerService)
	b.ReportAllocs()
	for b.Loop() {
		_ = svc.GenerateID()
	}
}

func BenchmarkGenerateUniqueID(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = GenerateUniqueID()
	}
}

func BenchmarkShortenURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	b.ReportAllocs()
	var i int
	for b.Loop() {
		if _, _, err := svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/page%d", i), "bench-user"); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	// pre-populate
	ids := make([]string, 200)
	for i := range ids {
		id, _, err := svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/pre%d", i), "u")
		if err != nil {
			b.Fatal(err)
		}
		ids[i] = id
	}
	b.ReportAllocs()
	var i int
	for b.Loop() {
		if _, _, err := svc.GetOriginalURL(ctx, ids[i%len(ids)]); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkShortenURLBatch(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	batch := make([]schemasshortener.RequestBatchURLSchema, 20)
	b.ReportAllocs()
	var i int
	for b.Loop() {
		for j := range batch {
			batch[j] = schemasshortener.RequestBatchURLSchema{
				CorrelationID: fmt.Sprintf("id-%d-%d", i, j),
				OriginalURL:   fmt.Sprintf("https://example.com/%d/%d", i, j),
			}
		}
		if _, err := svc.ShortenURLBatch(ctx, batch, "bench-user"); err != nil {
			b.Fatal(err)
		}
		i++
	}
}

func BenchmarkGetAllShortenerURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	for i := 0; i < 200; i++ {
		if _, _, err := svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/u%d", i), "bench-user"); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := svc.GetAllShortenerURL(ctx, "bench-user"); err != nil {
			b.Fatal(err)
		}
	}
}
