# 概要
得られたデータをプロジェクト別に分類するため、`projects.yaml` の設定ファイルで
プロジェクト名と正規表現をマッチングさせます。browserはtitle、terminalはcwdでマッチします。
具体的には、`/stats` の出力として以下のように返します。
projectsでプロジェクトごとの合算を出力し、project_othersでOtherの内訳を表示します。

```json
  {
    "browser_active_span": {
      "Codex CLI 通知設定": 24,
      "GitHub": 15,
      "Google カレンダー - 2026年 1月 4日の週": 0,
      "読書 – Todoist": 2
    },
    "terminal_command": {},
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


# プロジェクトマッチング設定ファイル `projects.yaml`

```yaml
projects:
  - name: 開発
    match:
      browser:
        title:
          - ".*GitHub.*"
          - ".*Codex.*"
      terminal:
        cwd:
          - ".*/dev/.*"

  - name: 企画
    match:
      browser:
        title:
          - ".*カレンダー.*"
          - ".*Todoist.*"
      terminal:
        cwd:
          - ".*/planning/.*"
```
