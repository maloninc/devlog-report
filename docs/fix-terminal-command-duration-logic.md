# 概要
terminal の長時間起動（例: claude/vim）によって日跨ぎの実行時間が記録され、実働より大きく計上される問題に対応する。schema_version を 2 に上げ、terminal の end_ts を start_ts に固定する方針で、集計ロジック（CWD 単位の MIN〜MAX 差分）は維持する。

# 問題点
- v1 の terminal_command は end_ts が実行終了時刻になっており、長時間起動で日跨ぎの差分が巨大化する。
- 例えば、claudeコマンドを 1/10 9:00 ~ 1/11 10:00 まで起動した場合、通常は、1/10 18:00ごろには作業を終了して、帰宅しているが claudeコマンド自体は終了せず、翌朝になってから終了するという状態になる。このような状況下では、claudeの起動時間を記録することに意味がない。

# 対応策
- schema_version=2 を全イベントに適用（browser も v2 に統一）
- terminal_command（v2）は end_ts を start_ts と同一にする
- 集計方式は変更しない（CWD 単位 MIN〜MAX 差分を継続）
- こうすることで、 先の例では、 claude の起動時刻だけが記録され、他のコマンドの記録によって作業記録の代替とする考えたを採用する。

# 検討事項
- 案1: 長時間起動コマンドの除外（例: claude/vim を記録対象外）
  - claude|vim|nvim|less|man|ssh|tmux|top|htop などを記録対象外にする。
  - 棄却理由: 除外リストの保守が必要で漏れやすい。新しい対話型コマンドに弱い。
- 案2: end_ts を start_ts に固定（terminal のみ）
  - 採用理由: 長時間起動の影響を確実に消せる。集計方式を維持できる。
- 案3: terminal の集計ロジックをイベントごとの合計に変更
  - 棄却理由: 集計の意味が変わり、既存の「作業継続の代理」という前提が崩れる。
- 案4: 上限時間でクランプ（server 側 or zsh 側）
  - 例: terminal_command の継続が N 分を超えたら N 分に丸める、または 0 扱いで無視。
  - 実際の「働いた時間」からの乖離を最小化しやすい。
  - 閾値は 10〜30 分が無難。対話型コマンドに限らず「放置」も吸収できる。
- 案5: 日付またぎ分割（server 側）
  - start_ts と end_ts が日付をまたぐ場合、日付境界で分割して当日分のみ計上。
  - “翌日への持ち越し” を抑制できるが、長時間放置自体は残る。
  - 既存集計ロジックへの影響が小さい。
- 案6: 日跨ぎのみ end_ts を start_ts に丸める
  - 棄却理由: 実装は可能だが判断条件が増え、v1/v2 併存時の挙動が分かりにくくなる。
- 次点（効果はあるが複雑）: アイドル検知で自動終了（zsh 側）
  - ZLE のキー入力やターミナル出力をフックして「最終操作」時刻を持つ。
  - 一定時間無操作なら “実質終了” とみなして end_ts を固定。

# マイグレーション
- サーバ起動時に毎回チェックを行い、`schema_version=1` のレコードが存在する場合のみマイグレーションを実行する。
- マイグレーション内容:
  - `schema_version=1` かつ type=terminal_command のレコードを対象に end_ts を start_ts に書き換える。
  - 全レコードを schema_version を 2 に更新し、二重実行されないようにする。

# 実装計画
* [x] マイグレーション検出と実行を追加
   - 起動時に `schema_version=1` の有無をチェック
   - ある場合のみ `schema_version=1` の全レコードを更新
* [x] マイグレーション更新内容を実装
   - 更新対象1: `schema_version=1` かつ type=terminal_command のレコード
   - 更新内容1: `end_ts = start_ts`
   - 更新対象2: 全レコード
   - 更新内容2: `schema_version=2`
   - 2回目以降は無変更で終了（冪等）
* [x] 送信側の schema_version を 2 に統一
   - zsh フックの `schema_version` を 2 に変更
   - terminal_command は送信時点で `end_ts = start_ts` をセット
   - browser 側も `schema_version=2` に統一（仕様変更なし）
* [ ] 互換性・エラー・レスポンス形式の確認
   - 互換性: 既存 API 仕様（/events 受理、/stats 出力）を維持
   - エラー: `schema_version` / 時刻フォーマットのバリデーションは現状維持
   - レスポンス形式: JSON/Markdown の出力変更なし
* [ ] 回帰確認
   - `/stats?date=YYYY-MM-DD` の md/json 出力が変わらないこと
   - terminal 集計が CWD の MIN〜MAX 差分であること
   - v1 データが存在する場合にのみマイグレーションが走ること
* [ ] 受け入れ手順の追加
   - v1 の `terminal_command` を 1 件投入 → 再起動後に `schema_version=2` かつ `end_ts=start_ts` に更新されること
   - v2 の terminal/browser を投入 → /stats の出力が従来通りであること
