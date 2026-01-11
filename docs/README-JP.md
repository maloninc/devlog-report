# DevLog Report とは？

DevLog Report は、macOS の行動ログ（Chrome のページタイトル/URL / zsh コマンド）から「1日の仕事配分」を推定する、**ローカル完結のエンジニア向けタイムトラッキング**仕組みです。  
このREADMEは、**「Chromeで見ていたページ（title + URL）」** と **「zshで実行したコマンド」** をローカルに記録し、`projects.yaml`（仕事ラベルとマッチングルール）を用いて **「どの仕事にどれだけ時間を割いたか」** を推定・可視化する設計方針をまとめたものです。

---

# 目的

- 1日の行動を **作業ブロック** として復元し、
  - どのプロジェクト／仕事（ラベル）に
  - どれくらい時間を使ったか
  - 何をしていたか（根拠URL/コマンド）
  を推定して日次レポート化する。
- 可能な限り **ローカル完結**（ログサーバもlocalhostで動かす）。
- 「ブラウザ滞在時間」など、履歴DBだけでは弱い部分を **拡張機能で補強**する。

---

# 前提・環境

- OS: macOS
- ブラウザ: Google Chrome（Manifest V3 拡張）
- ターミナル環境: zsh
- ログ収集先: ローカルログサーバ（localhost）
- プロジェクト定義: `projects.yaml` に以下を保持している前提
  - プロジェクト名（例: `project-alpha`, `ops`, `recruiting` など）
  - browserのtitle、terminalのcwdに対する正規表現ルール

---

# 全体アーキテクチャ（概要）

```
[Chrome Extension]  ──HTTP──►  [Local Log Server]  ─►  [SQLite]
      │                                  │
      └─(active span生成)                └─(日次集計/推論ジョブ)
[zsh hooks]         ──HTTP──►              │
                                         ▼
                               [Daily Report (Markdown/HTML)]
```

## 収集するログの柱
1. **ブラウザ（Chrome拡張）**
   - 「いつ、どのページを（title + URL）、アクティブに見ていたか」を区間（span）として記録
2. **ターミナル（zshフック）**
   - 「いつ、どのコマンドを、どのディレクトリで、実行したか」を記録

---

# 実装方針

## 1. ローカルログサーバ

## 役割
- Chrome拡張 / zshフック からのイベントを受け取り、永続化する。
- 収集時のネットワーク失敗は少ない想定だが、各クライアント側にバッファを持たせる。

## 推奨要件
- 受信: `POST /events`（JSON）
- 返信: 200/4xx/5xx（クライアントがリトライ判断）
- 永続化:
  - 推奨: SQLite（後で集計・検索しやすい）
- セキュリティ:
  - バインドは `127.0.0.1` のみに限定

- 受信: `GET /stats?date=YYYY-MM-DD`（UTC）
- 返信: dateで指定された日付について、SQLite上のデータから統計情報を返す

---

## 2. Chrome拡張（Manifest V3）

## 目的
- ブラウザ履歴DBでは弱い「滞在時間」を、拡張側で **アクティブ区間（active span）** として確定して送る。

## 収集したいイベント（最小セット）
- アクティブタブ切替（見始め）
- URL遷移（ページ移動）
- ChromeウィンドウのフォーカスIN/OUT（見ていない時間を切る）
- idle/locked（席を外した時間を切る）

## 実装方針
- イベント駆動でspanを確定

## span生成の基本ロジック（例）
- 「現在アクティブなページ（title + URL）」を `current_span` として保持
- 以下のイベントで `current_span` を確定して送信し、新しいspanを開始
  - タブ切替
  - URL遷移（コミット）
  - ウィンドウフォーカスOUT / idle / lock
- spanの最小フィールド:
  - `title`, `start_ts`, `end_ts`, `url`

---

## 3. zsh でのコマンド記録（拡張ではなくフックで実装）

## 方針
ターミナルはChromeのような「拡張エコシステム」が一般的ではないため、**zshフック（precmd）**で「コマンド境界」を確実に取る。

## 収集したい情報（最小セット）
- 実行時刻
- 実行コマンド文字列
- カレントディレクトリ（cwd）

## フックの概念
- `precmd`: コマンド実行後（次のプロンプトが出る直前）

> 補足: zshの `EXTENDED_HISTORY` を有効にすると、履歴に開始時刻と経過時間を埋め込めるが、
> 「書き出しタイミング」や「セッション跨ぎ」の癖があるため、**リアルタイム送信（フック）が基本**。

---

# イベントスキーマ（例）

## 共通フィールド
- `event_id`: UUID
- `source`: `chrome` / `zsh`
- `schema_version`: 例 `1`

## 1) browser_active_span
```json
{
  "type": "browser_active_span",
  "source": "chrome",
  "event_id": "uuid",
  "start_ts": "2026-01-03T10:00:00+09:00",
  "end_ts":   "2026-01-03T10:12:34+09:00",
  "url": "https://example.com/path",
  "title": "Example",
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
  "command": "git status",
}
```

---

# devlogd API

## POST /events

### Request
- Content-Type: `application/json`
- Body: イベントJSON

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

# 日次レポートの出力イメージ（例）

- 日付: 2026-01-03
- ラベル別時間
  - project-alpha: 3h 20m
  - ops: 1h 10m
  - recruiting: 40m
- 根拠（上位）
  - project-alpha:
    - Title: `GitHub`, `Jira`
    - CMD: `pytest ...`, `docker compose up`, `git rebase ...`
- サマリ（自然言語）
  - 午前は project-alpha の開発とテスト、午後は ops の対応、その後 recruiting の候補者確認…等

---

# 最小実装ロードマップ

1. ローカルログサーバ（`POST /events` + SQLite保存）
2. Chrome拡張で `browser_active_span` を送る
3. zshフックで `terminal_command` を送る（失敗時はバッファ）
4. 日次集計（ラベル別時間 + 根拠抽出）
5. `projects.yaml` を用いたラベル付け（正規表現ルール）

---

# projects.yaml の推奨フォーマット（例）

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

---

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

# ライセンス
MIT
