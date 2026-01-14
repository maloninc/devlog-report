# 概要
terminal の集計において、同一プロジェクトに属する複数の CWD がある場合、現在は「各CWDの MIN〜MAX を合算」しているため過大計上になる。これを「プロジェクト単位で MIN(start_ts)〜MAX(end_ts) を取る」方式へ変更する。browser は現行の集計方式を維持する。

# 問題点
- 例えば Project=bacon に対して CWD=/Projects/bacon と /Projects/digi-infra-tools/base_3items_extractor が紐づく場合、両方の MIN〜MAX を合算すると過大になる。

# 仕様
- 集計対象の日付フィルタは現状維持（`date(start_ts, 'localtime') = ?`）。
- terminal は「プロジェクト単位の MIN〜MAX」方式で時間を算出する。
- browser は既存の集計方式（title 単位の duration 合算）を維持する。
- drill down（`/stats?date=...&project=...`）の合計時間は、terminal はプロジェクト単位の MIN〜MAX、browser は既存の合算方式を使う。
- drill down の明細テーブルは現状維持し、terminal のみ各行に **MIN, MAX, duration** の3つを表示する。
  - そのため、明細の合計とヘッダの**duration合計**が一致しないケースがあるが仕様として許容する
  - その代わり、MIN, MAXも表示されるため重複期間を判定できるようにする意図
- Other は例外として、terminal も browser も従来の「合算方式」を使う。
  - Project Summary の Other は Others List の合計と一致させる。

# 出力の期待（例）
- `/stats?date=2026-01-13` の Project Summary では、Project=bacon の時間は
  - terminal: bacon + 3items_extractor の **合算ではなく**
  - Project=bacon に属する terminal イベント全体の MIN(start_ts)〜MAX(end_ts) の期間の経過時間
  - browser は既存の title 単位の duration 合算
  - terminal + browser の合計が Project=bacon の時間
- `/stats?date=2026-01-13&project=bacon` のヘッダ合計は terminal は MIN〜MAX、browser は合算
- `/stats?date=2026-01-13` の Project Summary では、Other の時間は
  - terminal / browser ともに合算方式
  - Others List の合計と一致する

# 実装計画
* [x] terminal のプロジェクト集計を「プロジェクト単位の MIN〜MAX」に変更
   - 対象: Project Summary の集計ロジック
   - terminal: プロジェクトに属する全 CWD のイベントから MIN(start_ts) と MAX(end_ts) を取る
   - browser は既存の title 単位 duration 合算を維持
* [x] drill down の合計計算を「terminal は MIN〜MAX、browser は合算」に変更
   - terminal の表示行（CWD 別）に MIN / MAX / duration を追加表示する
   - browser の表示行は現状維持（duration のみ）
   - ヘッダ合計は terminal の MIN〜MAX + browser の合算
* [x] Other の集計を合算方式にする
   - Project Summary の Other は terminal/browser ともに合算
   - Others List の合計と一致すること
