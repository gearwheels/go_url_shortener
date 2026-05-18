package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	"github.com/gearwheels/go_url_shortener/internal/service"
	repo "github.com/gearwheels/go_url_shortener/repositories"
)

func initBenchHandler(b *testing.B) {
	b.Helper()
	if service.Shortener == nil {
		service.Shortener = service.NewShortenerService(repo.NewRepoShortener())
	}
}

func BenchmarkShortenHandler(b *testing.B) {
	initBenchHandler(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := strings.NewReader(fmt.Sprintf("https://example.com/bench%d", i))
		req := httptest.NewRequest(http.MethodPost, "/", body)
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "bench-user"))
		rr := httptest.NewRecorder()
		ShortenHandler(rr, req)
	}
}

func BenchmarkJSONShortenHandler(b *testing.B) {
	initBenchHandler(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		body := strings.NewReader(fmt.Sprintf(`{"url":"https://example.com/json%d"}`, i))
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "bench-user"))
		rr := httptest.NewRecorder()
		JSONShortenHandler(rr, req)
	}
}

func BenchmarkRedirectHandler(b *testing.B) {
	initBenchHandler(b)
	// pre-populate 100 URLs
	ids := make([]string, 100)
	for i := range ids {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(fmt.Sprintf("https://example.com/redir%d", i)))
		req.Header.Set("Content-Type", "text/plain")
		req = req.WithContext(logrequest.ContextWithUserID(req.Context(), "bench-user"))
		rr := httptest.NewRecorder()
		ShortenHandler(rr, req)
		// extract id from response body
		body := strings.TrimSpace(rr.Body.String())
		if idx := strings.LastIndex(body, "/"); idx >= 0 {
			ids[i] = body[idx+1:]
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/"+ids[i%len(ids)], nil)
		rr := httptest.NewRecorder()
		RedirectHandler(rr, req)
	}
}
