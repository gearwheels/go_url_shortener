// Package audit реализует паттерн «Наблюдатель» для аудита событий сервиса.
// Auditor — субъект; FileObserver и HTTPObserver — конкретные наблюдатели.
// Пример подключения:
//
//	a := audit.NewAuditor()
//	a.Subscribe(audit.NewFileObserver("/var/log/audit.log"))
//	a.Subscribe(audit.NewHTTPObserver("https://collector.example.com/events"))
//	a.Notify(audit.Event{Action: "shorten", URL: "https://example.com"})
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

// Event описывает единицу аудита: действие пользователя над URL.
// Поле Ts заполняется автоматически при вызове Auditor.Notify.
type Event struct {
	Ts     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id,omitempty"`
	URL    string `json:"url"`
}

// Observer — интерфейс наблюдателя. Реализуйте его для подключения
// произвольного приёмника событий аудита.
type Observer interface {
	Notify(event Event)
}

// Auditor — субъект паттерна «Наблюдатель».
// Хранит список наблюдателей и доставляет им каждое событие.
// Безопасен при nil-получателе: вызов методов на nil не паникует.
type Auditor struct {
	observers []Observer
}

// NewAuditor возвращает новый пустой Auditor без наблюдателей.
func NewAuditor() *Auditor {
	return &Auditor{}
}

// Subscribe добавляет наблюдателя в список рассылки.
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

// FileObserver записывает события аудита в файл построчно (режим append).
// Каждая строка — JSON-объект типа Event.
type FileObserver struct {
	path string
}

// NewFileObserver создаёт FileObserver, пишущий в файл по пути path.
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

// HTTPObserver отправляет каждое событие аудита на удалённый HTTP-сервер
// методом POST с Content-Type: application/json.
type HTTPObserver struct {
	url string
}

// NewHTTPObserver создаёт HTTPObserver, отправляющий события на указанный URL.
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
