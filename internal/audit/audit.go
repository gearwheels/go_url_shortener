package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// Event — событие аудита
type Event struct {
	Ts     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}

// Observer — интерфейс наблюдателя
type Observer interface {
	Notify(event Event)
}

// Auditor — субъект паттерна «Наблюдатель», рассылает события всем подписчикам
type Auditor struct {
	observers []Observer
}

func NewAuditor() *Auditor {
	return &Auditor{}
}

func (a *Auditor) Subscribe(obs Observer) {
	a.observers = append(a.observers, obs)
}

// Notify рассылает событие всем наблюдателям. Nil-безопасен.
func (a *Auditor) Notify(event Event) {
	if a == nil {
		return
	}
	event.Ts = time.Now().Unix()
	for _, obs := range a.observers {
		obs.Notify(event)
	}
}

// FileObserver — записывает события аудита в файл (append)
type FileObserver struct {
	path string
}

func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

func (f *FileObserver) Notify(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("audit: marshal error", "error", err)
		return
	}
	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error("audit: open file error", "error", err)
		return
	}
	defer file.Close()
	fmt.Fprintln(file, string(data))
}

// HTTPObserver — отправляет событие аудита на удалённый сервер методом POST
type HTTPObserver struct {
	url string
}

func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{url: url}
}

func (h *HTTPObserver) Notify(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Error("audit: marshal error", "error", err)
		return
	}
	resp, err := http.Post(h.url, "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Error("audit: http send error", "error", err)
		return
	}
	defer resp.Body.Close()
}
