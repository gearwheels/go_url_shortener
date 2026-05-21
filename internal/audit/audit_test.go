package audit

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// --- helpers ---

func tempFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "audit.log")
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// --- FileObserver ---

func TestFileObserver_CreatesFile(t *testing.T) {
	path := tempFile(t)
	obs := NewFileObserver(path)
	obs.Notify(Event{TS: 1, Action: "shorten", URL: "https://example.com"})

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to be created: %v", err)
	}
}

func TestFileObserver_WritesValidJSON(t *testing.T) {
	path := tempFile(t)
	obs := NewFileObserver(path)
	obs.Notify(Event{TS: 111, Action: "shorten", UserID: "u1", URL: "https://a.com"})

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var e Event
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if e.Action != "shorten" {
		t.Errorf("action: want shorten, got %s", e.Action)
	}
	if e.UserID != "u1" {
		t.Errorf("user_id: want u1, got %s", e.UserID)
	}
	if e.URL != "https://a.com" {
		t.Errorf("url: want https://a.com, got %s", e.URL)
	}
}

func TestFileObserver_AppendsNewLines(t *testing.T) {
	path := tempFile(t)
	obs := NewFileObserver(path)
	obs.Notify(Event{Action: "shorten", URL: "https://a.com"})
	obs.Notify(Event{Action: "follow", URL: "https://b.com"})
	obs.Notify(Event{Action: "shorten", URL: "https://c.com"})

	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	actions := []string{"shorten", "follow", "shorten"}
	for i, line := range lines {
		var e Event
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("line %d invalid JSON: %v", i, err)
		}
		if e.Action != actions[i] {
			t.Errorf("line %d: action want %s, got %s", i, actions[i], e.Action)
		}
	}
}

func TestFileObserver_OmitsEmptyUserID(t *testing.T) {
	path := tempFile(t)
	obs := NewFileObserver(path)
	obs.Notify(Event{Action: "follow", URL: "https://x.com"})

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &raw); err != nil {
		t.Fatalf("json: %v", err)
	}
	if _, ok := raw["user_id"]; ok {
		t.Error("user_id should be omitted when empty")
	}
}

func TestFileObserver_BadPath_NoPanic(t *testing.T) {
	obs := NewFileObserver("/no/such/directory/audit.log")
	obs.Notify(Event{Action: "shorten", URL: "https://x.com"})
}

// --- HTTPObserver ---

func TestHTTPObserver_SendsPOST(t *testing.T) {
	var (
		mu       sync.Mutex
		received [][]byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll: %v", err)
		}
		mu.Lock()
		received = append(received, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL)
	obs.Notify(Event{TS: 999, Action: "shorten", UserID: "u2", URL: "https://example.org"})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 request, got %d", len(received))
	}

	var e Event
	if err := json.Unmarshal(received[0], &e); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if e.Action != "shorten" {
		t.Errorf("action: want shorten, got %s", e.Action)
	}
	if e.UserID != "u2" {
		t.Errorf("user_id: want u2, got %s", e.UserID)
	}
	if e.URL != "https://example.org" {
		t.Errorf("url: want https://example.org, got %s", e.URL)
	}
}

func TestHTTPObserver_BadURL_NoPanic(t *testing.T) {
	obs := NewHTTPObserver("http://127.0.0.1:1")
	obs.Notify(Event{Action: "follow", URL: "https://x.com"})
}

func TestHTTPObserver_ContentTypeJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type: want application/json, got %s", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL)
	obs.Notify(Event{Action: "shorten", URL: "https://x.com"})
}

// --- Auditor ---

type mockObserver struct {
	mu     sync.Mutex
	events []Event
}

func (m *mockObserver) Notify(event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockObserver) received() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

func TestAuditor_NotifiesAllObservers(t *testing.T) {
	a := NewAuditor()
	obs1 := &mockObserver{}
	obs2 := &mockObserver{}
	a.Subscribe(obs1)
	a.Subscribe(obs2)

	a.Notify(Event{Action: "shorten", URL: "https://x.com"})

	if len(obs1.received()) != 1 {
		t.Errorf("obs1: expected 1 event, got %d", len(obs1.received()))
	}
	if len(obs2.received()) != 1 {
		t.Errorf("obs2: expected 1 event, got %d", len(obs2.received()))
	}
}

func TestAuditor_SetsTimestamp(t *testing.T) {
	a := NewAuditor()
	obs := &mockObserver{}
	a.Subscribe(obs)

	before := time.Now().Unix()
	a.Notify(Event{Action: "shorten", URL: "https://x.com"})
	after := time.Now().Unix()

	events := obs.received()
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	ts := events[0].TS
	if ts < before || ts > after {
		t.Errorf("TS %d out of range [%d, %d]", ts, before, after)
	}
}

func TestAuditor_NilSafe(t *testing.T) {
	var a *Auditor
	a.Notify(Event{Action: "shorten", URL: "https://x.com"})
}

func TestAuditor_NoObservers_NoPanic(t *testing.T) {
	a := NewAuditor()
	a.Notify(Event{Action: "follow", URL: "https://x.com"})
}

func TestAuditor_EventFieldsPassedThrough(t *testing.T) {
	a := NewAuditor()
	obs := &mockObserver{}
	a.Subscribe(obs)

	a.Notify(Event{Action: "follow", UserID: "uid-99", URL: "https://z.com"})

	events := obs.received()
	if len(events) != 1 {
		t.Fatalf("expected 1 event")
	}
	e := events[0]
	if e.Action != "follow" {
		t.Errorf("action: want follow, got %s", e.Action)
	}
	if e.UserID != "uid-99" {
		t.Errorf("user_id: want uid-99, got %s", e.UserID)
	}
	if e.URL != "https://z.com" {
		t.Errorf("url: want https://z.com, got %s", e.URL)
	}
}

func TestAuditor_WithFileObserver(t *testing.T) {
	path := tempFile(t)
	a := NewAuditor()
	a.Subscribe(NewFileObserver(path))

	a.Notify(Event{Action: "shorten", UserID: "u3", URL: "https://integration.com"})

	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in file, got %d", len(lines))
	}
	var e Event
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("json: %v", err)
	}
	if e.Action != "shorten" || e.UserID != "u3" || e.URL != "https://integration.com" {
		t.Errorf("unexpected event: %+v", e)
	}
	if e.TS == 0 {
		t.Error("TS must be set by Auditor.Notify")
	}
}

func TestAuditor_WithHTTPObserver(t *testing.T) {
	var mu sync.Mutex
	var gotEvent Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("ReadAll: %v", err)
		}
		var e Event
		if err := json.Unmarshal(body, &e); err != nil {
			t.Errorf("Unmarshal: %v", err)
		}
		mu.Lock()
		gotEvent = e
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAuditor()
	a.Subscribe(NewHTTPObserver(srv.URL))
	a.Notify(Event{Action: "follow", UserID: "u4", URL: "https://remote.com"})

	mu.Lock()
	defer mu.Unlock()
	if gotEvent.Action != "follow" {
		t.Errorf("action: want follow, got %s", gotEvent.Action)
	}
	if gotEvent.URL != "https://remote.com" {
		t.Errorf("url: want https://remote.com, got %s", gotEvent.URL)
	}
}
