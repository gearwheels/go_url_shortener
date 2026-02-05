package logrequest

import (
	"compress/gzip"
	"net/http"
	"strings"
)

func RequestDataZip() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Распаковка входящего запроса
			if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
				reader, err := gzip.NewReader(r.Body)
				if err == nil {
					defer reader.Close()
					r.Body = reader
					r.Header.Del("Content-Encoding")
					r.ContentLength = -1
				}
			}

			// 2. Проверяем поддержку сжатия клиентом
			acceptEncoding := r.Header.Get("Accept-Encoding")
			supportsGzip := strings.Contains(acceptEncoding, "gzip")
			acceptContentType := r.Header.Get("Content-Type")
			allowedContentType := strings.Contains(acceptContentType, "application/json") || strings.Contains(acceptContentType, "text/html")
			// allowedContentType := strings.Contains(acceptContentType, "text/html")

			// 3. Создаем обертку для ResponseWriter
			var cw *compressedWriter
			if supportsGzip && allowedContentType {
				cw = &compressedWriter{
					ResponseWriter: w,
					supportsGzip:   supportsGzip,
				}
			}

			if cw != nil {
				defer cw.Close()
				next.ServeHTTP(cw, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}

type compressedWriter struct {
	http.ResponseWriter
	supportsGzip bool
	gzipWriter   *gzip.Writer
	wroteHeader  bool
}

func (cw *compressedWriter) WriteHeader(statusCode int) {
	if !cw.wroteHeader {
		cw.wroteHeader = true

		// Проверяем content type
		contentType := cw.Header().Get("Content-Type")
		shouldCompress := cw.supportsGzip &&
			(strings.Contains(contentType, "application/json") ||
				strings.Contains(contentType, "text/html"))

		if shouldCompress {
			// Устанавливаем заголовки сжатия
			cw.Header().Set("Content-Encoding", "gzip")
			cw.Header().Set("Vary", "Accept-Encoding")
			cw.Header().Del("Content-Length")

			// Создаем gzip writer
			cw.gzipWriter = gzip.NewWriter(cw.ResponseWriter)
		}
	}

	cw.ResponseWriter.WriteHeader(statusCode)
}

func (cw *compressedWriter) Write(b []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	if cw.gzipWriter != nil {
		return cw.gzipWriter.Write(b)
	}

	return cw.ResponseWriter.Write(b)
}

func (cw *compressedWriter) Close() error {
	if cw.gzipWriter != nil {
		return cw.gzipWriter.Close()
	}
	return nil
}

func (cw *compressedWriter) Flush() {
	if cw.gzipWriter != nil {
		cw.gzipWriter.Flush()
	}
	if flusher, ok := cw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
