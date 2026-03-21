# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 未着手

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- [x] ADR-018: メトリクス体系の再設計（merged PR スコープ, タスク種別分類, LEFT JOIN 修正, review_comments）
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示
- ADR-017: 設計/実装セッション分離の自動ディスパッチ
  - 関連 ADR: [ADR-017](docs/adr/017-automated-implementation-session-dispatch.md)
  - [ ] `/dispatch` skill が TODO.md の未着手セクションから全タスクを検出できる
  - [ ] `/dispatch` skill がタスクごとに worktree + tmux session + Claude Code を起動できる
  - [ ] TODO.md の受け入れ条件を含む初期プロンプトが `tmux send-keys` で渡される（ADR 有無による分岐なし）
  - [ ] `--dry-run` 引数で実際の起動なしに対象タスクを確認できる
  - [ ] 既に worktree/ブランチが存在するタスクはスキップされる

## 進行中

（なし）
