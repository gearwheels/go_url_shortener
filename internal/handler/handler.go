// Package handler содержит HTTP-обработчики сервиса сокращения URL.
//
// Маршруты:
//
//	POST /                      — ShortenHandler         (text/plain)
//	POST /api/shorten           — JSONShortenHandler      (application/json)
//	POST /api/shorten/batch     — ShortenBatchHandler     (application/json)
//	GET  /{id}                  — RedirectHandler
//	GET  /ping                  — CheckDBStatus
//	GET  /api/user/urls         — UserURL
//	GET  /api/internal/stats    — StatsHandler
//	DELETE /api/user/urls       — DeleteBatchHandler
package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gearwheels/go_url_shortener/internal/audit"
	"github.com/gearwheels/go_url_shortener/internal/config"
	logrequest "github.com/gearwheels/go_url_shortener/internal/middleware"
	schemasshortener "github.com/gearwheels/go_url_shortener/internal/schemas"
	"github.com/gearwheels/go_url_shortener/internal/service"
)

// Auditor рассылает события аудита наблюдателям; устанавливается в main.go.
var Auditor *audit.Auditor

// trustedCIDR хранит разобранный CIDR доверенной подсети; nil — подсеть не задана.
var trustedCIDR *net.IPNet

// InitTrustedSubnet разбирает CIDR один раз при старте.
// Если cidr пустой — trustedCIDR остаётся nil и StatsHandler будет отдавать 403.
func InitTrustedSubnet(cidr string) error {
	if cidr == "" {
		return nil
	}
	_, parsed, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	trustedCIDR = parsed
	return nil
}

// TrustedSubnetMiddleware проверяет, что X-Real-IP входит в trustedCIDR.
// Возвращает 403, если подсеть не задана или IP вне диапазона.
func TrustedSubnetMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if trustedCIDR == nil {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		clientIP := net.ParseIP(r.Header.Get("X-Real-IP"))
		if clientIP == nil || !trustedCIDR.Contains(clientIP) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ShortenHandler обрабатывает POST / с телом text/plain.
// Принимает оригинальный URL, возвращает короткую ссылку в теле ответа.
//
//	201 Created  — ссылка создана впервые.
//	409 Conflict — ссылка для этого URL уже существует (возвращает существующую).
func ShortenHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "text/plain" {
		http.Error(w, "Unsupported Media Type. Expected text/plain",
			http.StatusUnsupportedMediaType)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalURL := strings.TrimSpace(string(body))

	if originalURL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(originalURL, "http://") &&
		!strings.HasPrefix(originalURL, "https://") {
		originalURL = "http://" + originalURL
	}

	userID, err := logrequest.GetUserID(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	id, inserted, err := service.Shortener.ShortenURL(r.Context(), originalURL, userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL", "error", err)
		return
	}

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Location", shortenedURL)
	if inserted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusConflict)
	}

	fmt.Fprint(w, shortenedURL)

	slog.Info("Created short URL", "short_url", shortenedURL, "original", originalURL)
	Auditor.Notify(audit.Event{Action: "shorten", UserID: userID, URL: originalURL})
}

// JSONShortenHandler обрабатывает POST /api/shorten с телом application/json.
// Тело запроса: {"url":"https://..."}.
// Тело ответа: {"result":"http://base/<id>"}.
//
//	201 Created  — ссылка создана впервые.
//	409 Conflict — ссылка для этого URL уже существует.
func JSONShortenHandler(w http.ResponseWriter, r *http.Request) {

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Unsupported Media Type. Expected application/json",
			http.StatusUnsupportedMediaType)
		return
	}
	var buf bytes.Buffer
	var request schemasshortener.RequestSchema
	var response schemasshortener.ResponseSchema

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf.Bytes(), &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if request.URL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(request.URL, "http://") &&
		!strings.HasPrefix(request.URL, "https://") {
		request.URL = "http://" + request.URL
	}

	userID, err := logrequest.GetUserID(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	id, inserted, err := service.Shortener.ShortenURL(r.Context(), request.URL, userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL", "error", err)
		return
	}

	shortenedURL := fmt.Sprintf("%s%s", config.AppConfig.BaseURL, id)
	response.Result = shortenedURL
	resp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to marshal JSON", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if inserted {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusConflict)
	}

	w.Write(resp)

	slog.Info("Created short URL", "short_url", shortenedURL, "original", request.URL)
	Auditor.Notify(audit.Event{Action: "shorten", UserID: userID, URL: request.URL})
}

// RedirectHandler обрабатывает GET /{id}.
// Извлекает оригинальный URL по короткому идентификатору и перенаправляет клиента.
//
//	307 Temporary Redirect — успешное перенаправление.
//	410 Gone               — ссылка удалена.
//	404 Not Found          — ссылка не найдена.
func RedirectHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из пути (убираем ведущий слэш)
	id := strings.TrimPrefix(r.URL.Path, "/")

	if id == "" {
		http.Error(w, "Send POST request with URL in body as text/plain to shorten URL", http.StatusBadRequest)
		return
	}

	originalURL, delFlag, err := service.Shortener.GetOriginalURL(r.Context(), id)
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	if delFlag {
		w.WriteHeader(http.StatusGone)
	} else {
		w.WriteHeader(http.StatusTemporaryRedirect)
	}

	userID, err := logrequest.GetUserID(r.Context())
	if err != nil {
		slog.Warn("RedirectHandler: userID not in context", "error", err)
	}
	Auditor.Notify(audit.Event{Action: "follow", UserID: userID, URL: originalURL})
	slog.Info("Redirecting", "id", id, "location", originalURL)
}

// CheckDBStatus обрабатывает GET /ping.
// Проверяет доступность базы данных и возвращает 200 OK или 500 Internal Server Error.
func CheckDBStatus(w http.ResponseWriter, r *http.Request) {

	db, err := sql.Open("pgx", config.AppConfig.DatabaseDsn)
	if err != nil {
		slog.Error("Ошибка открытия соединения", "error", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		http.Error(w, "Short URL not found", http.StatusInternalServerError)
		return
	} else {
		w.WriteHeader(http.StatusOK)
		slog.Info("Data base alive!")
		return
	}
}

// ShortenBatchHandler обрабатывает POST /api/shorten/batch.
// Принимает массив объектов {"correlation_id","original_url"} и возвращает
// массив {"correlation_id","short_url"}.
//
//	201 Created — все ссылки созданы.
func ShortenBatchHandler(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "Unsupported Media Type. Expected application/json",
			http.StatusUnsupportedMediaType)
		return
	}
	var buf bytes.Buffer
	var batchURL []schemasshortener.RequestBatchURLSchema

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf.Bytes(), &batchURL); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(batchURL) == 0 {
		http.Error(w, "Batch of URL cannot be empty", http.StatusBadRequest)
		return
	}

	userID, err := logrequest.GetUserID(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	response, err := service.Shortener.ShortenURLBatch(r.Context(), batchURL, userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to shorten URL batch", "error", err)
		return
	}

	resp, err := json.Marshal(response)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to marshal JSON", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	w.Write(resp)
}

// userURLItem — элемент JSON-ответа для GET /api/user/urls.
type userURLItem struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// UserURL обрабатывает GET /api/user/urls.
// Возвращает список всех коротких ссылок аутентифицированного пользователя.
//
//	200 OK         — список ссылок в формате [{"short_url","original_url"},...].
//	204 No Content — у пользователя нет ссылок.
//	401 Unauthorized — пользователь не аутентифицирован.
func UserURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, err := logrequest.GetUserID(ctx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	userURLs, err := service.Shortener.GetAllShortenerURL(ctx, userID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to get all users short url", "error", err)
		return
	}

	if len(userURLs) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	items := make([]userURLItem, 0, len(userURLs))
	for _, u := range userURLs {
		items = append(items, userURLItem{
			ShortURL:    fmt.Sprintf("%s%s", config.AppConfig.BaseURL, u.ShortURL),
			OriginalURL: u.URL,
		})
	}
	resp, err := json.Marshal(items)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to marshal JSON", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// DeleteBatchHandler возвращает http.HandlerFunc для DELETE /api/user/urls.
// tasksDelCh — канал, в который передаются задания воркеру фонового удаления.
func DeleteBatchHandler(tasksDelCh chan<- schemasshortener.Task) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		DeleteBatch(w, r, tasksDelCh)
	}
}

// DeleteBatch — реализация DELETE /api/user/urls.
// Помечает ссылки как удалённые и отправляет задания на физическое удаление в tasksDelCh.
//
//	202 Accepted — задание принято, удаление произойдёт асинхронно.
func DeleteBatch(w http.ResponseWriter, r *http.Request, tasksDelCh chan<- schemasshortener.Task) {
	ctx := r.Context()
	userID, err := logrequest.GetUserID(ctx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	var batchID []string
	var buf bytes.Buffer

	// читаем тело запроса
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = json.Unmarshal(buf.Bytes(), &batchID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(batchID) == 0 {
		http.Error(w, "Batch of ID cannot be empty", http.StatusBadRequest)
		return
	}

	err = service.Shortener.MarkOnDeleteBatch(ctx, userID, batchID)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("Failed to get all users short url", "error", err)
		return
	}

	for _, item := range batchID {
		tasksDelCh <- schemasshortener.Task{UserID: userID, Data: item}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
}

// StatsHandler обрабатывает GET /api/internal/stats.
// Доступен только через TrustedSubnetMiddleware — проверка IP уже выполнена.
// Возвращает {"urls":<int>,"users":<int>}.
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	urls, users, err := service.Shortener.GetStats(r.Context())
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		slog.Error("StatsHandler: GetStats failed", "error", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(struct {
		URLs  int `json:"urls"`
		Users int `json:"users"`
	}{urls, users})
}
