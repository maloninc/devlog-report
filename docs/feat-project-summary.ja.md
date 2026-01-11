# 概要
`/stats` を改良して、 以下のようにプロジェクトへの時間投下状況をMarkdownで表示するようにしてください。
また、従来通りJSONで返すモードとして、 `/stats?mode=json` を残してください。

# Markdownの出力仕様
- 「時間降順」 にプロジェクト毎に表示する
- 秒から切り上げで分に変換する

```md
# Project Summary
| Project   | Time(min) |
| --------- | --------- |
| Project A | 120       |
| Project B |  90       |
| Other     |  60       |

# Others List
| Others    | Type     | Time(min) |
| --------- | ---------| --------- |
| Hogehoge  | browser  |  90       |
| Fugafuga  | browser  |  60       |
| Brabra    | terminal |  20       |
```


# 実装計画
* [x] `/stats` のレスポンスモードを追加  
   - `mode=json` のときは従来通り JSON を返す  
   - `mode` 未指定 or `mode=md` のときは Markdown を返す  
   - Content-Type: JSON は `application/json`、Markdown は `text/markdown; charset=utf-8`
* [x] Markdown 生成ロジックを追加  
   - 秒→分は「切り上げ」(ceil) で変換  
   - `projects` を時間降順で並べて「Project Summary」表を出力  
   - `project_others` を1つの表にまとめ、`Type` 列で `browser/terminal` を表現  
* [x] 並び順ルールを実装  
   - 時間降順、同値なら名前昇順  
* [x] 既存 JSON 仕様の保持  
   - `/stats?mode=json` で現在の JSON を返す  
   - 既存の `projects` / `project_others` を活用
* [x] README に `/stats` の `mode` 仕様と Markdown 例を追記

## 追加対応
- Markdown の列幅を固定  
  - Project / Others は 60 文字幅  
  - Type は 8 文字幅  
  - Time(min) は 9 文字幅で数値は右寄せ  
- 全角幅を考慮したパディング  
  - `github.com/mattn/go-runewidth` を使用
