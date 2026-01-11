# 概要
drill-downのエンドポイントを `/drill-down` から `/stats` へ変更する

# 変更の背景
従来の `/drill-down` のエンドポイントは、エンドポイントとしてわかりやすいが、
実際の操作として、 `/stats` にアクセスしてから `/drill-down` へのアクセスとなり、
エンドポイントの編集が若干煩雑になる。

例えば、

`curl 'localhost:8787/stats?date=2026-01-05'` を実行した後、プロジェクト名「企画」の
ドリルダウンを見るために、
`curl 'localhost:8787/drill-down?date=2026-01-05&project=企画'` を実行することになるが、
この場合の変更点は、 

- `/stats`から`/drill-down`への変更
- `&project=企画`の追加

という２点になる。

これに対して、もしも、 ドリルダウンの機能がエンドポイント `/stats` で実装されていれば
以下のようにシンプルになる。

```shell
% curl 'localhost:8787/stats?date=2026-01-05'
% curl 'localhost:8787/stats?date=2026-01-05&project=企画'
```

# 仕様
エンドポイント `/stats` のパラメータに `project` が指定されている場合には、
/stats の projects 合計の内訳について、指定のプロジェクトの内容をドリルダウン表示する

# 実装方針
- /drill-down は 削除する （互換性は気にしない。まだリリースしていないので）
- /stats?project=... の 空データ時 は既存のドリルダウン同様に 404 + "not found" でOK
- /stats?project=... で mode 未指定のとき は Markdown でOK
- /stats の通常集計とドリルダウンの JSONレスポンス形式は維持でOK


# 実装計画
* [*] `/stats` にドリルダウン分岐を追加
   - `project` 指定時はドリルダウン出力（既存の drill-down ロジックを流用）
   - `mode` は md/json、未指定は md
   - 空データは `404` + `"not found"`（text/plain）
* [*] `/drill-down` ルートを削除
   - 互換性は考慮しないためルーティング登録を削除
* [*] `/stats` 既存挙動の回帰確認
   - `project` 未指定時の JSON/Markdown が変わらないことを確認
