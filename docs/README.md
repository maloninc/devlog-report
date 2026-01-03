# What is DevLog Report?

DevLog Report is a **local-only time tracking system for engineers** that estimates how you spent your day based on macOS activity logs (Chrome URLs and zsh commands).
This README summarizes the design for recording **URLs viewed in Chrome** and **commands executed in zsh** locally, then using `project.md` (labels and descriptions) to estimate and visualize **how much time went to each project**.

---

# Goals

- Reconstruct a day's activity as **work blocks**, then estimate and report:
  - which project / label
  - how much time was spent
  - what was done (evidence URLs/commands)
- Keep everything **local-only** (log server runs on localhost).
- Compensate for weak signals in browser history (e.g., dwell time) with a **Chrome extension**.

---

# Assumptions / Environment

- OS: macOS
- Browser: Google Chrome (Manifest V3 extension)
- Terminal: zsh
- Log sink: local log server (localhost)
- Project definition: `project.md` includes
  - project name (e.g. `project-alpha`, `ops`, `recruiting`)
  - project ID (e.g. `pj-001`, `pj-xyz`)
  - label description (natural language)
  - optional URL/command rules (recommended)

---

# Architecture (Overview)

```
[Chrome Extension]  ──HTTP──►  [Local Log Server]  ─►  [SQLite]
      │                                  │
      └─(active span creation)           └─(daily aggregation/inference)
[zsh hooks]         ──HTTP──►              │
                                         ▼
                               [Daily Report (Markdown/HTML)]
```

## Log sources
1. **Browser (Chrome extension)**
   - Records spans of **active URL viewing**
2. **Terminal (zsh hooks)**
   - Records **commands executed** and **current directory**

---

# Implementation Plan

## 1. Local log server

## Role
- Receive events from Chrome extension / zsh hooks and persist them.
- Network failures are unlikely, but clients should buffer if needed.

## Recommended requirements
- Receive: `POST /events` (JSON)
- Response: 200/4xx/5xx (clients decide retry)
- Persistence:
  - Recommended: SQLite (easier to aggregate/search)
- Security:
  - Bind only to `127.0.0.1`

- Receive: `GET /stats?date=YYYY-MM-DD` (UTC)
- Response: returns stats for the specified date from SQLite

---

## 2. Chrome Extension (Manifest V3)

## Purpose
- Browser history alone is weak for dwell time; the extension determines **active spans** and sends them.

## Minimum events to capture
- Active tab switch (start viewing)
- URL navigation (page change)
- Chrome window focus in/out (exclude unfocused time)
- idle/locked (exclude away time)

## Implementation concept
- Finalize spans on events

## Span generation logic (example)
- Keep current active URL as `current_span`
- Finalize `current_span` on:
  - tab switch
  - URL commit
  - window focus out / idle / lock
- Minimum fields for a span:
  - `title`, `start_ts`, `end_ts`, `url`

---

## 3. Command logging in zsh (via hooks)

## Approach
Terminal lacks a standard extension ecosystem, so use **zsh hooks (precmd)** to capture command boundaries.

## Minimum data to record
- execution timestamps
- command string
- current working directory (cwd)

## Hook concept
- `precmd`: after command execution (just before prompt)

> Note: zsh `EXTENDED_HISTORY` can embed start time and duration in history, but it has quirks with write timing and session boundaries. Real-time hooks are the primary approach.

---

# Event schema (examples)

## Common fields
- `event_id`: UUID
- `source`: `chrome` / `zsh`
- `schema_version`: e.g. `1`

## 1) browser_active_span
```json
{
  "type": "browser_active_span",
  "source": "chrome",
  "event_id": "uuid",
  "start_ts": "2026-01-03T10:00:00+09:00",
  "end_ts":   "2026-01-03T10:12:34+09:00",
  "url": "https://example.com/path",
  "title": "Example"
}
```

## 2) terminal_command
```json
{
  "type": "terminal_command",
  "source": "zsh",
  "event_id": "uuid",
  "start_ts": "2026-01-03T10:13:10+09:00",
  "end_ts":   "2026-01-03T10:13:12+09:00",
  "cwd": "/Users/me/repos/project-alpha",
  "command": "git status"
}
```

---

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

### Response
- `200 OK` / `400 Bad Request`
- Aggregated seconds

```json
{
  "terminal_command": {
    "/path/to/somewhere": 100,
    "/path/to/elsewhere": 120
  },
  "browser_active_span": {
    "https://example.com/path": 111,
    "https://hoge.com/path": 123
  }
}
```

---

# Daily report example (output)

- Date: 2026-01-03
- Time by label
  - project-alpha: 3h 20m
  - ops: 1h 10m
  - recruiting: 40m
- Evidence (top)
  - project-alpha:
    - URL: `github.com/...`, `jira.company/...`
    - CMD: `pytest ...`, `docker compose up`, `git rebase ...`
- Summary (natural language)
  - Morning: project-alpha development and testing. Afternoon: ops work, then recruiting candidate review.

---

# Minimal roadmap

1. Local log server (`POST /events` + SQLite persistence)
2. Chrome extension sends `browser_active_span`
3. zsh hook sends `terminal_command` (buffer on failure)
4. Daily aggregation (time by label + evidence extraction)
5. Labeling via `project.md` (rules → similarity)

---

# Recommended `project.md` format (example)

```md
# project-alpha
Project ID: pj-001
Description: Improve search quality.
repo: project-alpha.github.com
cwd: /repos/project-alpha

# ops
Project ID: pj-ops
Description: Production ops, incident response, monitoring.
repo: ops.github.com
cwd: /repos/ops
```

---

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

# License
MIT
