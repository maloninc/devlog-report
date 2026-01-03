package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	Type          string `json:"type"`
	Source        string `json:"source"`
	EventID       string `json:"event_id"`
	SchemaVersion int    `json:"schema_version"`

	StartTS string `json:"start_ts,omitempty"`
	EndTS   string `json:"end_ts,omitempty"`

	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`

	CWD     string `json:"cwd,omitempty"`
	Command string `json:"command,omitempty"`
}

func normalizeEvent(b []byte) (Event, error) {
	var ev Event
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil {
		return Event{}, err
	}

	if err := validateEvent(ev); err != nil {
		return Event{}, err
	}

	return ev, nil
}

func validateEvent(ev Event) error {
	if ev.Type == "" {
		return errors.New("type is required")
	}
	if ev.Source == "" {
		return errors.New("source is required")
	}
	if ev.EventID == "" {
		return errors.New("event_id is required")
	}
	if ev.SchemaVersion == 0 {
		return errors.New("schema_version is required")
	}
	if ev.StartTS == "" || ev.EndTS == "" {
		return errors.New("start_ts and end_ts are required")
	}
	if err := parseTime(ev.StartTS); err != nil {
		return errors.New("start_ts must be RFC3339")
	}
	if err := parseTime(ev.EndTS); err != nil {
		return errors.New("end_ts must be RFC3339")
	}

	switch ev.Type {
	case "browser_active_span":
		if ev.URL == "" {
			return errors.New("url is required for browser_active_span")
		}
		if ev.Title == "" {
			return errors.New("title is required for browser_active_span")
		}
	case "terminal_command":
		if ev.CWD == "" {
			return errors.New("cwd is required for terminal_command")
		}
		if ev.Command == "" {
			return errors.New("command is required for terminal_command")
		}
	default:
		return errors.New("unknown type")
	}

	return nil
}

func parseTime(value string) error {
	if value == "" {
		return errors.New("empty time")
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return nil
	}
	_, err := time.Parse(time.RFC3339, value)
	return err
}

func parseTimeValue(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, value)
}

type eventStore struct {
	db *sql.DB
}

func newEventStore(path string) (*eventStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &eventStore{db: db}, nil
}

func initSchema(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	event_id TEXT NOT NULL UNIQUE,
	type TEXT NOT NULL,
	source TEXT NOT NULL,
	schema_version INTEGER NOT NULL,
	start_ts TEXT NOT NULL,
	end_ts TEXT NOT NULL,
	url TEXT,
	title TEXT,
	cwd TEXT,
	command TEXT,
	payload TEXT NOT NULL,
	received_at TEXT NOT NULL
);
`)
	return err
}

func (s *eventStore) insert(ev Event, payload string) error {
	receivedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`
INSERT INTO events (
	event_id, type, source, schema_version, start_ts, end_ts,
	url, title, cwd, command, payload, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		ev.EventID, ev.Type, ev.Source, ev.SchemaVersion, ev.StartTS, ev.EndTS,
		ev.URL, ev.Title, ev.CWD, ev.Command, payload, receivedAt,
	)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return errors.New("event_id already exists")
	}
	return err
}

func (s *eventStore) terminalDurationsByCWD(date string) (map[string]int64, error) {
	rows, err := s.db.Query(`
SELECT cwd, MIN(start_ts), MAX(end_ts)
FROM events
WHERE type = 'terminal_command' AND date(start_ts) = ?
GROUP BY cwd
`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var cwd string
		var minStart string
		var maxEnd string
		if err := rows.Scan(&cwd, &minStart, &maxEnd); err != nil {
			return nil, err
		}
		startTime, err := parseTimeValue(minStart)
		if err != nil {
			return nil, err
		}
		endTime, err := parseTimeValue(maxEnd)
		if err != nil {
			return nil, err
		}
		secs := int64(endTime.Sub(startTime).Seconds())
		if secs < 0 {
			secs = 0
		}
		out[cwd] = secs
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *eventStore) browserDurationsByURL(date string) (map[string]int64, error) {
	rows, err := s.db.Query(`
SELECT url, start_ts, end_ts
FROM events
WHERE type = 'browser_active_span' AND date(start_ts) = ?
`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var url string
		var startStr string
		var endStr string
		if err := rows.Scan(&url, &startStr, &endStr); err != nil {
			return nil, err
		}
		startTime, err := parseTimeValue(startStr)
		if err != nil {
			return nil, err
		}
		endTime, err := parseTimeValue(endStr)
		if err != nil {
			return nil, err
		}
		secs := int64(endTime.Sub(startTime).Seconds())
		if secs < 0 {
			secs = 0
		}
		out[url] += secs
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *eventStore) close() error {
	return s.db.Close()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func main() {
	addr := envOr("DEVLOG_ADDR", "127.0.0.1:8787")
	dbPath := envOr("DEVLOG_DB_PATH", "./data/devlog.db")

	store, err := newEventStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open event store: %v", err)
	}
	defer func() {
		if err := store.close(); err != nil {
			log.Printf("failed to close store: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
			return
		}

		ev, err := normalizeEvent(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		payload := string(body)
		if err := store.insert(ev, payload); err != nil {
			if err.Error() == "event_id already exists" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist event"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "event_id": ev.EventID})
	})

	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		date := r.URL.Query().Get("date")
		if date == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date is required (YYYY-MM-DD, UTC)"})
			return
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date must be YYYY-MM-DD (UTC)"})
			return
		}

		terminal, err := store.terminalDurationsByCWD(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute terminal stats"})
			return
		}
		browser, err := store.browserDurationsByURL(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute browser stats"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"terminal_command":    terminal,
			"browser_active_span": browser,
		})
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("devlogd listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func envOr(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
