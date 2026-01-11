# What is DevLog Report?

DevLog Report is a **local-only time tracking system for engineers** that estimates how you spent your day from macOS activity logs (Chrome page titles/URLs and zsh commands).

---

# Environment / Assumptions

- OS: macOS
- Browser: Google Chrome (Manifest V3 extension)
- Terminal: zsh
- Log sink: local log server (localhost)
- Project definition: `projects.yaml` includes
  - project names (e.g. `project-alpha`, `ops`, `recruiting`)
  - regex rules for browser titles and terminal CWDs

---

# Logs Collected

1. **Browser (Chrome extension)**
   - Records **when and which page (title + URL)** was active, as spans
2. **Terminal (zsh hooks)**
   - Records **when and which command** was executed, and **in which directory**


# Build

## devlogd

```shell
cd server
go mod tidy
go build ./cmd/devlogd
```

## zsh hook

```shell
cd zsh
./install.sh
```

## Chrome extension

No build required. Load it into Chrome.

1. Open `chrome://extensions/`
2. Turn on Developer mode (top right)
3. Click "Load unpacked"
4. Select the `chrome/` directory


# Quickstart

## Start the server
```shell
cd server
nohup ./devlogd &
disown
```

## Show stats
```shell
curl 'localhost:8787/stats?date=2026-01-05'
```

## Drill down by project
```shell
curl 'localhost:8787/stats?date=2026-01-05&project=Project-A'
```

# devlogd API

## POST /events

### Request
- Content-Type: `application/json`
- Body: event JSON

```json
{
  "type": "browser_active_span",
  "source": "chrome",
  "event_id": "uuid",
  "schema_version": 1,
  "start_ts": "2026-01-03T10:00:00Z",
  "end_ts": "2026-01-03T10:12:34Z",
  "url": "https://example.com/path",
  "title": "Example"
}
```

```json
{
  "type": "terminal_command",
  "source": "zsh",
  "event_id": "uuid",
  "schema_version": 1,
  "start_ts": "2026-01-03T10:13:10Z",
  "end_ts": "2026-01-03T10:13:12Z",
  "cwd": "/Users/me/repos/project-alpha",
  "command": "git status"
}
```

### Response
- `200 OK` / `409 Conflict` / `400 Bad Request`

```json
{ "status": "ok", "event_id": "uuid" }
```

## GET /stats?date=YYYY-MM-DD

### Request
- Query: `date` is a UTC date
- Optional: `mode=json` (default is Markdown when omitted or `mode=md`)

### Response (JSON)
- `200 OK` / `400 Bad Request`
- Aggregated seconds (browser spans are grouped by title)

```json
{
  "terminal_command": {
    "/path/to/somewhere": 100,
    "/path/to/elsewhere": 120
  },
  "browser_active_span": {
    "Example": 111,
    "Hoge": 123
  },
  "projects": {
    "PJ-A": 4,
    "PJ-B": 3,
    "Other": 10
  },
  "project_others": {
    "browser": {
      "brabra": 2,
      "brabrabara": 3
    },
    "terminal": {
      "hoge": 2,
      "fuga": 3
    }
  }
}
```

### Response (Markdown)

```md
# Project Summary
| Project   | Time(min) |
| --------- | --------- |
| Project A | 120       |
| Project B |  90       |
| Other     |  60       |

# Others List
| Others    | Type     | Time(min) |
| --------- | -------- | --------- |
| Hogehoge  | browser  |  90       |
| Fugafuga  | browser  |  60       |
| Brabra    | terminal |  20       |
```

---

# Recommended `projects.yaml` format

```yaml
projects:
  - name: project-alpha
    match:
      browser:
        title:
          - ".*GitHub.*"
          - ".*Jira.*"
      terminal:
        cwd:
          - ".*/repos/project-alpha.*"

  - name: ops
    match:
      browser:
        title:
          - ".*Datadog.*"
      terminal:
        cwd:
          - ".*/repos/ops.*"
```

Use `DEVLOG_PROJECTS_PATH` to change the file location (default: `./projects.yaml`).

# License
MIT
