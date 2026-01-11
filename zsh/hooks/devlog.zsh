#!/usr/bin/env zsh

# DevLog Report zsh hook (preexec/precmd)

# Always (re)register hooks; remove existing ones to avoid duplicates.

typeset -g DEVLOG_LAST_CMD=""
typeset -g DEVLOG_LAST_START=""

devlog_now_rfc3339() {
  date -u '+%Y-%m-%dT%H:%M:%S.%3Z'
}

devlog_uuid() {
  /usr/bin/uuidgen
}

devlog_json_escape() {
  local s="$1"
  s="${s//\\/\\\\}"
  s="${s//\"/\\\"}"
  s="${s//$'\n'/\\n}"
  s="${s//$'\r'/\\r}"
  s="${s//$'\t'/\\t}"
  printf '%s' "$s"
}

devlog_build_payload() {
  local start_ts="$1"
  local end_ts="$2"
  local cwd="$3"
  local cmd="$4"

  local esc_cwd
  local esc_cmd
  esc_cwd="$(devlog_json_escape "$cwd")"
  esc_cmd="$(devlog_json_escape "$cmd")"

  printf '{"type":"terminal_command","source":"zsh","event_id":"%s","schema_version":2,"start_ts":"%s","end_ts":"%s","cwd":"%s","command":"%s"}' \
    "$(devlog_uuid)" \
    "$start_ts" \
    "$end_ts" \
    "$esc_cwd" \
    "$esc_cmd"
}

devlog_preexec() {
  DEVLOG_LAST_CMD="$1"
  DEVLOG_LAST_START="$(devlog_now_rfc3339)"
}

devlog_precmd() {
  if [[ -z "$DEVLOG_LAST_CMD" ]]; then
    return
  fi

  local end_ts
  end_ts="$DEVLOG_LAST_START"

  local payload
  payload="$(devlog_build_payload "$DEVLOG_LAST_START" "$end_ts" "$PWD" "$DEVLOG_LAST_CMD")"

  DEVLOG_LAST_CMD=""
  DEVLOG_LAST_START=""

  local endpoint="${DEVLOG_ENDPOINT:-http://127.0.0.1:8787/events}"
  curl -sS --max-time 1 --connect-timeout 1 \
    -H "Content-Type: application/json" \
    -d "$payload" \
    "$endpoint" >/dev/null 2>&1 &!
}

autoload -Uz add-zsh-hook
add-zsh-hook -d preexec devlog_preexec 2>/dev/null || true
add-zsh-hook -d precmd devlog_precmd 2>/dev/null || true
add-zsh-hook preexec devlog_preexec
add-zsh-hook precmd devlog_precmd
