# DevLog Report とは？

DevLog Report は、macOS の行動ログ（Chrome のページタイトル/URL / zsh コマンド）から「1日の仕事配分」を推定する、**ローカル完結のエンジニア向けタイムトラッキング**仕組みです。  

---

# 環境・前提

- OS: macOS
- ブラウザ: Google Chrome（Manifest V3 拡張）
- ターミナル環境: zsh
- ログ収集先: ローカルログサーバ（localhost）
- プロジェクト定義: `projects.yaml` に以下を保持している前提
  - プロジェクト名（例: `project-alpha`, `ops`, `recruiting` など）
  - browserのtitle、terminalのcwdに対する正規表現ルール

---

# 収集するログ
1. **ブラウザ（Chrome拡張）**
   - 「いつ、どのページを（title + URL）、アクティブに見ていたか」を区間（span）として記録
2. **ターミナル（zshフック）**
   - 「いつ、どのコマンドを、どのディレクトリで、実行したか」を記録


# ビルド方法

## devlogd

```shell
cd server
go mod tidy
go build ./cmd/devlogd
```

## zsh フック

```shell
cd zsh
./install.sh
```

## Chrome 拡張

ビルドは不要。Chrome に読み込むだけで動作します。

1. `chrome://extensions/` を開く  
2. 右上の「デベロッパーモード」を ON  
3. 「パッケージ化されていない拡張機能を読み込む」  
4. `chrome/` ディレクトリを選択


# クイックスタート

## サーバの起動
```shell
cd server
nohup ./devlogd &
disown
```

## 統計の表示
```shell
curl 'localhost:8787/stats?date=2026-01-05'
```

## 統計のプロジェクト単位のドリルダウン
```shell
curl 'localhost:8787/stats?date=2026-01-05&project=Project-A'
```

# devlogd API

## POST /events

### Request
- Content-Type: `application/json`
- Body: イベントJSON
  - terminal_command の `end_ts` は `start_ts` と同一にする

```json
{
  "type": "browser_active_span",
  "source": "chrome",
  "event_id": "uuid",
  "schema_version": 2,
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
  "schema_version": 2,
  "start_ts": "2026-01-03T10:13:10Z",
  "end_ts": "2026-01-03T10:13:10Z",
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
- Query: `date` は UTC の日付
- Optional: `mode=json`（省略 or `mode=md` で Markdown）

### Response（JSON）
- `200 OK` / `400 Bad Request`
- 秒単位の集計結果（ブラウザは title 単位）

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

### Response（Markdown）

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

# projects.yaml の推奨フォーマット

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

ファイルの場所は `DEVLOG_PROJECTS_PATH` で変更できます（デフォルト: `./projects.yaml`）。

# ライセンス
MIT
