package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"gopkg.in/yaml.v3"
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

	if ev.Type == "terminal_command" {
		ev.EndTS = ev.StartTS
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

type ProjectsConfig struct {
	Projects []ProjectConfig `yaml:"projects"`
}

type ProjectConfig struct {
	Name  string       `yaml:"name"`
	Match ProjectMatch `yaml:"match"`
}

type ProjectMatch struct {
	Browser  BrowserMatch  `yaml:"browser"`
	Terminal TerminalMatch `yaml:"terminal"`
}

type BrowserMatch struct {
	Title []string `yaml:"title"`
}

type TerminalMatch struct {
	CWD []string `yaml:"cwd"`
}

type compiledProject struct {
	name           string
	browserTitleRe []*regexp.Regexp
	terminalCwdRe  []*regexp.Regexp
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

func (s *eventStore) migrateSchemaV2() (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}

	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM events WHERE schema_version = 1 LIMIT 1`).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			_ = tx.Rollback()
			return false, nil
		}
		_ = tx.Rollback()
		return false, err
	}

	if _, err := tx.Exec(`
UPDATE events
SET end_ts = start_ts
WHERE schema_version = 1 AND type = 'terminal_command'
`); err != nil {
		_ = tx.Rollback()
		return false, err
	}

	if _, err := tx.Exec(`
UPDATE events
SET schema_version = 2
WHERE schema_version = 1
`); err != nil {
		_ = tx.Rollback()
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *eventStore) terminalDurationsByCWD(date string) (map[string]int64, error) {
	rows, err := s.db.Query(`
SELECT cwd, MIN(start_ts), MAX(end_ts)
FROM events
WHERE type = 'terminal_command' AND date(start_ts, 'localtime') = ?
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

func (s *eventStore) browserDurationsByTitle(date string) (map[string]int64, error) {
	rows, err := s.db.Query(`
SELECT title, url, start_ts, end_ts
FROM events
WHERE type = 'browser_active_span' AND date(start_ts, 'localtime') = ?
`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int64)
	for rows.Next() {
		var title string
		var url string
		var startStr string
		var endStr string
		if err := rows.Scan(&title, &url, &startStr, &endStr); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(title)
		if key == "" {
			key = url
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
		out[key] += secs
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

func writeMarkdown(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func writePlainText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func loadProjectsConfig(path string) (ProjectsConfig, error) {
	var cfg ProjectsConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func compileProjectMatchers(cfg ProjectsConfig) ([]compiledProject, error) {
	compiled := make([]compiledProject, 0, len(cfg.Projects))
	for _, project := range cfg.Projects {
		entry := compiledProject{name: project.Name}

		for _, pattern := range project.Match.Browser.Title {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			entry.browserTitleRe = append(entry.browserTitleRe, re)
		}

		for _, pattern := range project.Match.Terminal.CWD {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			entry.terminalCwdRe = append(entry.terminalCwdRe, re)
		}

		compiled = append(compiled, entry)
	}
	return compiled, nil
}

func classifyProjects(
	terminal map[string]int64,
	browser map[string]int64,
	cfg ProjectsConfig,
) (map[string]int64, map[string]map[string]int64, error) {
	projectTotals := make(map[string]int64)
	project_others := map[string]map[string]int64{
		"browser":  {},
		"terminal": {},
	}

	for _, project := range cfg.Projects {
		projectTotals[project.Name] = 0
	}

	const otherName = "Other"
	projectTotals[otherName] = 0

	compiled, err := compileProjectMatchers(cfg)
	if err != nil {
		return nil, nil, err
	}

	assignBrowser := func(title string, seconds int64) {
		for _, project := range compiled {
			for _, re := range project.browserTitleRe {
				if re.MatchString(title) {
					projectTotals[project.name] += seconds
					return
				}
			}
		}
		projectTotals[otherName] += seconds
		project_others["browser"][title] += seconds
	}

	assignTerminal := func(cwd string, seconds int64) {
		for _, project := range compiled {
			for _, re := range project.terminalCwdRe {
				if re.MatchString(cwd) {
					projectTotals[project.name] += seconds
					return
				}
			}
		}
		projectTotals[otherName] += seconds
		project_others["terminal"][cwd] += seconds
	}

	for title, seconds := range browser {
		assignBrowser(title, seconds)
	}

	for cwd, seconds := range terminal {
		assignTerminal(cwd, seconds)
	}

	return projectTotals, project_others, nil
}

type projectRow struct {
	name    string
	seconds int64
}

type otherRow struct {
	name    string
	typ     string
	seconds int64
}

const (
	markdownNameWidth = 60
	markdownTypeWidth = 8
	markdownTimeWidth = 9
)

type drillDownRow struct {
	name    string
	typ     string
	seconds int64
}

func ceilMinutes(seconds int64) int64 {
	if seconds <= 0 {
		return 0
	}
	return int64(math.Ceil(float64(seconds) / 60.0))
}

func drillDownRows(
	terminal map[string]int64,
	browser map[string]int64,
	cfg ProjectsConfig,
	projectName string,
) ([]drillDownRow, int64, bool, error) {
	const otherName = "Other"
	projectExists := projectName == otherName
	for _, project := range cfg.Projects {
		if project.Name == projectName {
			projectExists = true
			break
		}
	}
	if !projectExists {
		return nil, 0, false, nil
	}

	compiled, err := compileProjectMatchers(cfg)
	if err != nil {
		return nil, 0, false, err
	}

	matchBrowser := func(title string) string {
		for _, project := range compiled {
			for _, re := range project.browserTitleRe {
				if re.MatchString(title) {
					return project.name
				}
			}
		}
		return otherName
	}

	matchTerminal := func(cwd string) string {
		for _, project := range compiled {
			for _, re := range project.terminalCwdRe {
				if re.MatchString(cwd) {
					return project.name
				}
			}
		}
		return otherName
	}

	var rows []drillDownRow
	var total int64
	for title, seconds := range browser {
		if matchBrowser(title) == projectName {
			rows = append(rows, drillDownRow{name: title, typ: "browser", seconds: seconds})
			total += seconds
		}
	}
	for cwd, seconds := range terminal {
		if matchTerminal(cwd) == projectName {
			rows = append(rows, drillDownRow{name: cwd, typ: "terminal", seconds: seconds})
			total += seconds
		}
	}

	return rows, total, true, nil
}

func padRightWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) >= width {
		return runewidth.Truncate(value, width, "")
	}
	return value + strings.Repeat(" ", width-runewidth.StringWidth(value))
}

func padLeftWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(value) >= width {
		return runewidth.Truncate(value, width, "")
	}
	return strings.Repeat(" ", width-runewidth.StringWidth(value)) + value
}

func renderStatsMarkdown(
	projects map[string]int64,
	projectOthers map[string]map[string]int64,
) string {
	projectRows := make([]projectRow, 0, len(projects))
	for name, seconds := range projects {
		projectRows = append(projectRows, projectRow{name: name, seconds: seconds})
	}
	sort.Slice(projectRows, func(i, j int) bool {
		if projectRows[i].seconds != projectRows[j].seconds {
			return projectRows[i].seconds > projectRows[j].seconds
		}
		return projectRows[i].name < projectRows[j].name
	})

	otherRows := make([]otherRow, 0)
	for typ, items := range projectOthers {
		for name, seconds := range items {
			otherRows = append(otherRows, otherRow{name: name, typ: typ, seconds: seconds})
		}
	}
	sort.Slice(otherRows, func(i, j int) bool {
		if otherRows[i].seconds != otherRows[j].seconds {
			return otherRows[i].seconds > otherRows[j].seconds
		}
		if otherRows[i].name != otherRows[j].name {
			return otherRows[i].name < otherRows[j].name
		}
		return otherRows[i].typ < otherRows[j].typ
	})

	var b strings.Builder
	b.WriteString("# Project Summary\n\n")
	b.WriteString("| Project")
	b.WriteString(strings.Repeat(" ", markdownNameWidth-7))
	b.WriteString(" | Time(min) |\n")
	b.WriteString(strings.Repeat(" ", markdownTimeWidth-9))
	b.WriteString("| ")
	b.WriteString(strings.Repeat("-", markdownNameWidth))
	b.WriteString(" | ")
	b.WriteString(strings.Repeat("-", markdownTimeWidth))
	b.WriteString(" |\n")
	for _, row := range projectRows {
		b.WriteString("| ")
		b.WriteString(padRightWidth(row.name, markdownNameWidth))
		b.WriteString(" | ")
		b.WriteString(padLeftWidth(strconv.FormatInt(ceilMinutes(row.seconds), 10), markdownTimeWidth))
		b.WriteString(" |\n")
	}

	b.WriteString("\n")
	b.WriteString("# Others List\n\n")
	b.WriteString("| Others")
	b.WriteString(strings.Repeat(" ", markdownNameWidth-6))
	b.WriteString(" | Type")
	b.WriteString(strings.Repeat(" ", markdownTypeWidth-4))
	b.WriteString(" | Time(min")
	b.WriteString(strings.Repeat(" ", markdownTimeWidth-8))
	b.WriteString(") |\n")
	b.WriteString("| ")
	b.WriteString(strings.Repeat("-", markdownNameWidth))
	b.WriteString(" | ")
	b.WriteString(strings.Repeat("-", markdownTypeWidth))
	b.WriteString(" | ")
	b.WriteString(strings.Repeat("-", markdownTimeWidth))
	b.WriteString(" |\n")
	for _, row := range otherRows {
		b.WriteString("| ")
		b.WriteString(padRightWidth(row.name, markdownNameWidth))
		b.WriteString(" | ")
		b.WriteString(padRightWidth(row.typ, markdownTypeWidth))
		b.WriteString(" | ")
		b.WriteString(padLeftWidth(strconv.FormatInt(ceilMinutes(row.seconds), 10), markdownTimeWidth))
		b.WriteString(" |\n")
	}

	return b.String()
}

func renderDrillDownMarkdown(projectName string, totalSeconds int64, rows []drillDownRow) string {
	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(projectName)
	b.WriteString(" ")
	b.WriteString(strconv.FormatInt(ceilMinutes(totalSeconds), 10))
	b.WriteString(": Drill down\n\n")

	b.WriteString("| Title/CWD")
	b.WriteString(strings.Repeat(" ", markdownNameWidth-9))
	b.WriteString(" | Type")
	b.WriteString(strings.Repeat(" ", markdownTypeWidth-4))
	b.WriteString(" | Time(min")
	b.WriteString(strings.Repeat(" ", markdownTimeWidth-8))
	b.WriteString(") |\n")
	b.WriteString("| ")
	b.WriteString(strings.Repeat("-", markdownNameWidth))
	b.WriteString(" | ")
	b.WriteString(strings.Repeat("-", markdownTypeWidth))
	b.WriteString(" | ")
	b.WriteString(strings.Repeat("-", markdownTimeWidth))
	b.WriteString(" |\n")
	for _, row := range rows {
		b.WriteString("| ")
		b.WriteString(padRightWidth(row.name, markdownNameWidth))
		b.WriteString(" | ")
		b.WriteString(padRightWidth(row.typ, markdownTypeWidth))
		b.WriteString(" | ")
		b.WriteString(padLeftWidth(strconv.FormatInt(ceilMinutes(row.seconds), 10), markdownTimeWidth))
		b.WriteString(" |\n")
	}

	return b.String()
}

func main() {
	addr := envOr("DEVLOG_ADDR", "127.0.0.1:8787")
	dbPath := envOr("DEVLOG_DB_PATH", "./data/devlog.db")
	projectsPath := envOr("DEVLOG_PROJECTS_PATH", "./projects.yaml")

	store, err := newEventStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open event store: %v", err)
	}
	if migrated, err := store.migrateSchemaV2(); err != nil {
		log.Fatalf("failed to migrate events: %v", err)
	} else if migrated {
		log.Printf("migration: schema_version 1 -> 2 completed")
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date is required (YYYY-MM-DD, local time)"})
			return
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date must be YYYY-MM-DD (local time)"})
			return
		}

		projectName := r.URL.Query().Get("project")

		terminal, err := store.terminalDurationsByCWD(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute terminal stats"})
			return
		}
		browser, err := store.browserDurationsByTitle(date)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to compute browser stats"})
			return
		}

		cfg, err := loadProjectsConfig(projectsPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load projects config"})
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			cfg = ProjectsConfig{}
		}

		if projectName != "" {
			rows, totalSeconds, projectExists, err := drillDownRows(terminal, browser, cfg, projectName)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid projects config"})
				return
			}
			if !projectExists || len(rows) == 0 {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}

			sort.Slice(rows, func(i, j int) bool {
				if rows[i].seconds != rows[j].seconds {
					return rows[i].seconds > rows[j].seconds
				}
				if rows[i].name != rows[j].name {
					return rows[i].name < rows[j].name
				}
				return rows[i].typ < rows[j].typ
			})

			mode := r.URL.Query().Get("mode")
			if mode == "" || mode == "md" {
				body := renderDrillDownMarkdown(projectName, totalSeconds, rows)
				writeMarkdown(w, http.StatusOK, body)
				return
			}
			if mode != "json" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be 'json' or 'md'"})
				return
			}

			type drillDownItem struct {
				TitleCWD string `json:"title/cwd"`
				Type     string `json:"type"`
				Seconds  int64  `json:"seconds"`
			}

			list := make([]drillDownItem, 0, len(rows))
			for _, row := range rows {
				list = append(list, drillDownItem{
					TitleCWD: row.name,
					Type:     row.typ,
					Seconds:  row.seconds,
				})
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"name":    projectName,
				"seconds": totalSeconds,
				"list":    list,
			})
			return
		}

		projectsTotals := map[string]int64{}
		project_others := map[string]map[string]int64{
			"browser":  {},
			"terminal": {},
		}
		if err == nil {
			projectsTotals, project_others, err = classifyProjects(terminal, browser, cfg)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid projects config"})
				return
			}
		} else {
			projectsTotals["Other"] = 0
			for _, seconds := range terminal {
				projectsTotals["Other"] += seconds
			}
			for _, seconds := range browser {
				projectsTotals["Other"] += seconds
			}
			project_others["browser"] = browser
			project_others["terminal"] = terminal
		}

		mode := r.URL.Query().Get("mode")
		if mode == "" || mode == "md" {
			body := renderStatsMarkdown(projectsTotals, project_others)
			writeMarkdown(w, http.StatusOK, body)
			return
		}
		if mode != "json" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be 'json' or 'md'"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"terminal_command":    terminal,
			"browser_active_span": browser,
			"projects":            projectsTotals,
			"project_others":      project_others,
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
