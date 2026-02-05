package logrequest

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func TestRequestDataZip_DecompressRequest(t *testing.T) {
	plainBody := `{"url":"https://example.com"}`
	gzipped, err := gzipBytes([]byte(plainBody))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}

	var receivedBody string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	})

	mw := RequestDataZip()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(gzipped))
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if receivedBody != plainBody {
		t.Errorf("Expected handler to receive decompressed body %q, got %q", plainBody, receivedBody)
	}
}

func TestRequestDataZip_CompressResponse(t *testing.T) {
	jsonBody := `{"result":"http://localhost:8080/abc123"}`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(jsonBody))
	})

	mw := RequestDataZip()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if enc := rr.Header().Get("Content-Encoding"); enc != "gzip" {
		t.Errorf("Expected Content-Encoding: gzip, got %q", enc)
	}
	if vary := rr.Header().Get("Vary"); vary != "Accept-Encoding" {
		t.Errorf("Expected Vary: Accept-Encoding, got %q", vary)
	}

	// Распаковываем ответ
	gr, err := gzip.NewReader(rr.Body)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()
	decoded, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(decoded) != jsonBody {
		t.Errorf("Expected decoded body %q, got %q", jsonBody, string(decoded))
	}
}

func TestRequestDataZip_NoCompressionWithoutAcceptEncoding(t *testing.T) {
	jsonBody := `{"result":"http://localhost:8080/xyz"}`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jsonBody))
	})

	mw := RequestDataZip()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	// Без Accept-Encoding: gzip

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if enc := rr.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("Expected no Content-Encoding, got %q", enc)
	}
	if rr.Body.String() != jsonBody {
		t.Errorf("Expected body %q, got %q", jsonBody, rr.Body.String())
	}
}

func TestRequestDataZip_NoCompressionForPlainTextRequest(t *testing.T) {
	// Запрос с Content-Type: text/plain — middleware не использует compressedWriter
	// для ответа, т.к. allowedContentType проверяет Content-Type запроса.
	// При text/plain запросе allowedContentType = false, ответ идёт без обёртки.
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("http://localhost:8080/abc"))
	})

	mw := RequestDataZip()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Accept-Encoding", "gzip")

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	// Ответ text/plain, запрос text/plain — allowedContentType false, сжатия нет
	if enc := rr.Header().Get("Content-Encoding"); enc != "" {
		t.Errorf("Expected no Content-Encoding for text/plain flow, got %q", enc)
	}
	if rr.Body.String() != "http://localhost:8080/abc" {
		t.Errorf("Expected plain body, got %q", rr.Body.String())
	}
}

func TestRequestDataZip_PlainBodyPassedThrough(t *testing.T) {
	body := `{"url":"https://test.com"}`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != body {
			http.Error(w, "body mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestDataZip()
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	mw(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}
