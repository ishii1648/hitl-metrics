# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 実装待ち

- ADR-019: backfill を Stop hook に移行
  - 関連 ADR: [ADR-019](docs/adr/019-backfill-stop-hook-migration.md)
  - [ ] Stop hook から `hitl-metrics backfill && hitl-metrics sync-db` が実行される
  - [ ] cursor（`hitl-metrics-state.json`）により前回処理済み以降のエントリのみが走査される
  - [ ] Phase 2（マージ判定）が一定間隔経過時のみ実行される
  - [ ] `configs/launchd/com.user.hitl-metrics-sync.plist` が削除される
  - [ ] `docs/setup.md` から launchd 手順が削除され hooks 設定に置換される

## 検討中

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示

## 進行中

（なし）
