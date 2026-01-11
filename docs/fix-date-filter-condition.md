# 概要
これまで、 `?date` パラメータで指定する日付は UTC であったが、 UTC以外の国で実行すると正しくフィルタリングできない問題が発生。
対策として `?date` の解釈を“UTC固定”から“実行環境のローカルタイムゾーン基準”に切り替える

# 互換性について
互換性の考慮は不要（まだリリース前のため）

# 実装方針
`/stats` の `?date` フィルタは SQLite の `date()` 関数で `start_ts` を日付比較しているため、
`date(start_ts)` をローカルタイムゾーン基準へ寄せる。具体的には `date(start_ts, 'localtime')` を使い、
`?date=YYYY-MM-DD` の日付を「実行環境のローカル日付」として解釈する。
入力形式は従来どおり `YYYY-MM-DD` を維持し、エラーメッセージの UTC 文言もローカル基準に合わせて更新する。

# 実装計画
* [x] `/stats` の日付フィルタをローカルタイムゾーン基準に変更
   - `terminalDurationsByCWD` / `browserDurationsByTitle` の SQL を `date(start_ts, 'localtime') = ?` に置換
   - フィルタ対象は `start_ts` を維持（集計対象の変更はしない）
* [x] `?date` バリデーション/エラー文言の更新
   - 入力形式は `YYYY-MM-DD` のまま
   - エラーメッセージの「UTC」表記をローカル基準へ修正
* [ ] リグレッションチェック
   - `?date` 未指定時の 400 応答が変わらない
   - `mode=md/json` のレスポンス形式が変わらない
* [ ] 受け入れ確認手順を追加
   - `curl 'localhost:8787/stats?date=YYYY-MM-DD'`
   - ローカル日付基準でフィルタされた結果が返ること
