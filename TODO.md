# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 実装待ち

（なし）

## 未着手

- `pr_urls` の追加順を保持して sync-db の採用 PR を安定化
  - [ ] `sessionindex.Update` / `UpdateByBranch` が URL を辞書順ソートせず既存順 + 追加順を保持する
  - [ ] `sync-db` の「最後の PR URL を採用」仕様と実装が一致する
  - [ ] 複数 PR URL を持つ session-index のテストを追加する
- `session-index.jsonl` の書き戻しを atomic 化
  - [ ] `WriteAll` が一時ファイルへ書き込んでから rename する
  - [ ] 書き込み失敗時に既存の `session-index.jsonl` が truncate されない
  - [ ] `backfill` / `update` 系の既存テストが通る
- Stop hook の `hitl-metrics` PATH 依存をなくす
  - [ ] `hitl-metrics hook stop` が hook 実行環境の PATH 差異で失敗しない
  - [ ] `backfill` と `sync-db` を同一プロセスで直接呼ぶ、または install 時に絶対パスを登録する方針を決める
  - [ ] 失敗時のログがユーザーに原因を追える内容になっている
- ローカル検証環境と CI の再現性を改善
  - [ ] `go test ./...` の SQLite 関連テストが macOS arm64 ローカル環境で安定して実行できる
  - [ ] `go test -race ./...` がローカルで実行不能な場合の代替手順を docs に明記する
  - [ ] Go バージョン・toolchain・modernc.org/sqlite の制約を整理する

## 検討中

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示

## 進行中
