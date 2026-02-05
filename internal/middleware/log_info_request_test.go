package logrequest

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestLogger_Passthrough(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := RequestLogger(logger)
	wrapped := mw(next)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("Expected handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if body := rr.Body.String(); body != "ok" {
		t.Errorf("Expected body %q, got %q", "ok", body)
	}

	// Лог должен содержать информацию о запросе
	logOut := logBuf.String()
	if !strings.Contains(logOut, "HTTP request") {
		t.Error("Expected log to contain 'HTTP request'")
	}
	if !strings.Contains(logOut, "/test") {
		t.Error("Expected log to contain request URI /test")
	}
	if !strings.Contains(logOut, "status=") {
		t.Error("Expected log to contain status")
	}
}

func TestRequestLogger_StatusCodeAndSize(t *testing.T) {
	logBuf := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	mw := RequestLogger(logger)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader("{}"))
	rr := httptest.NewRecorder()

	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	logOut := logBuf.String()
	if !strings.Contains(logOut, "status=201") {
		t.Errorf("Expected log to contain status=201, got: %s", logOut)
	}
}

func TestRequestLogger_DifferentMethods(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelInfo}))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestLogger(logger)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()
			mw(next).ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Expected status %d for %s, got %d", http.StatusOK, method, rr.Code)
			}
		})
	}
}
