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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.GenerateID()
	}
}

func BenchmarkGenerateUniqueID(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateUniqueID()
	}
}

func BenchmarkShortenURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/page%d", i), "bench-user")
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	// pre-populate
	ids := make([]string, 200)
	for i := range ids {
		id, _, _ := svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/pre%d", i), "u")
		ids[i] = id
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.GetOriginalURL(ctx, ids[i%len(ids)])
	}
}

func BenchmarkShortenURLBatch(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	batch := make([]schemasshortener.RequestBatchURLSchema, 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j := range batch {
			batch[j] = schemasshortener.RequestBatchURLSchema{
				CorrelationID: fmt.Sprintf("id-%d-%d", i, j),
				OriginalURL:   fmt.Sprintf("https://example.com/%d/%d", i, j),
			}
		}
		b.StartTimer()
		_, _ = svc.ShortenURLBatch(ctx, batch, "bench-user")
	}
}

func BenchmarkGetAllShortenerURL(b *testing.B) {
	svc := newBenchService()
	ctx := context.Background()
	for i := 0; i < 200; i++ {
		_, _, _ = svc.ShortenURL(ctx, fmt.Sprintf("https://example.com/u%d", i), "bench-user")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.GetAllShortenerURL(ctx, "bench-user")
	}
}
