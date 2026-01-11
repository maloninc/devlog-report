# 概要
 /stats の projects 合計の内訳について、特定のプロジェクトの内容をドリルダウン表示する。 
 例えば、以下のように表示する。


## エントリーポイント

GET /drill-down?date=yyyy-mm-dd&project=Project-A

GET /drill-down?date=yyyy-mm-dd&project=Project-A&mode=json

### パラメータ
date: UTCの日付（必須）
project: 特定したいプロジェクト名称（必須）
mode: json or md, default値はmd

## 出力内容(markdown)
- 時間の降順でソートして表示する（同秒数の場合の順序は任意でOK）
- Title/CWDは、
    - Typeがbrowser_active_spanの場合、Title
    - Typeがterminal_commandの場合、CWD
    を表示する
- Time(min) の算出は秒から分に切り上げる
- 入力の browser_active_span / terminal_command を browser / terminal に変換する
- 該当データがない（空データの場合）、ステータス404 + bodyにプレーンテキストで "not found"を返す


```md
# Project-A 170: Drill down

| Title/CWD | Type     | Time(min) |
| --------- | ---------| --------- |
| Hogehoge  | browser  |  90       |
| Fugafuga  | browser  |  60       |
| Brabra    | terminal |  20       |
```

## 出力内容(json)

```json
{
    "name": "Project-A",
    "seconds": 10200,
    "list": [
        {
            "title/cwd": "Hogehoge",
            "type": "browser",
            "seconds": 5400
        },
        {
            "title/cwd": "Fugafuga",
            "type": "browser",
            "seconds": 3600
        },
        {
            "title/cwd": "Brabra",
            "type": "terminal",
            "seconds": 1200
        }
    ]
}
```

# 実装計画
* [x] データ取得と抽出ロジックを実装
    - 既存の `terminalDurationsByCWD(date)` / `browserDurationsByTitle(date)` を利用
    - `projects.yaml` を `loadProjectsConfig` → `classifyProjects` で読み込み・分類
    - 指定 `project` にマッチした browser/terminal のみを抽出する関数を追加
* [x] レスポンス整形（Markdown/JSON）
    - 降順ソート（同秒数は任意）
    - Title/CWD と type 変換（browser_active_span → browser, terminal_command → terminal）
    - 合計秒数と list を組み立て、Markdown は分単位に切り上げ
* [x] ルーティング追加とエラーハンドリング
    - `GET /drill-down` を追加（必須: `date`, `project`）
    - `mode` は `md` / `json`（default: `md`）
    - 該当データが空の場合は `404` + `"not found"`（プレーンテキスト）
