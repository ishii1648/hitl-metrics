# 使い方・動作説明

hitl-metrics の動作の仕組み、日常の運用、トラブルシューティングについて説明します。セットアップ手順は [setup.md](setup.md) を参照してください。

## データフロー

```
hooks → ~/.claude/*.jsonl|log → hitl-metrics backfill → hitl-metrics sync-db → SQLite → Grafana
```

## hook の役割

| hook | トリガー | 出力先 |
|------|----------|--------|
| `hitl-metrics hook session-start` | セッション開始時 | `~/.claude/session-index.jsonl` |
| `hitl-metrics hook permission-request` | Permission UI 表示時 | `~/.claude/logs/permission.log` |
| `hitl-metrics hook pre-tool-use` | ツール実行前 | `~/.claude/logs/last-tool-*` |
| `hitl-metrics hook stop` | セッション終了時 | `~/.claude/hitl-metrics.db`（backfill + sync-db） |
| `hitl-metrics hook todo-cleanup` | セッション開始時（main） | `TODO.md` → `CHANGELOG.md` |

セッション開始時にインデックスが記録され、Permission UI 表示時にログが記録されます。セッション終了時に PR URL 補完と SQLite DB 同期が自動実行されます。

## データ同期の仕組み

Stop hook がセッション終了時に `hitl-metrics backfill` → `hitl-metrics sync-db` を実行します。

- **backfill Phase 1（URL 補完）**: `pr_urls` が空のセッションに対し、`gh pr list` で PR URL を取得
- **backfill Phase 2（マージ判定）**: 未マージ PR の `is_merged` と `review_comments` を更新（1時間間隔）
- **sync-db**: JSONL/log → SQLite 変換（`~/.claude/hitl-metrics.db` を生成）

cursor（`~/.claude/hitl-metrics-state.json`）により前回処理済み以降のエントリのみが走査されるため、高速に完了します。

## 日常の運用

Stop hook が登録済みであれば、セッション終了時に `backfill` と `sync-db` が自動実行されます。手動で即時更新する場合:

```fish
hitl-metrics backfill
hitl-metrics sync-db
```

ダッシュボードは `http://localhost:3000`（デフォルト）でアクセスできます。

## CLI コマンド

```
hitl-metrics install                               hooks を ~/.claude/settings.json に登録
hitl-metrics backfill [--recheck]                  PR URL の一括補完
hitl-metrics sync-db                               JSONL/log → SQLite 変換
hitl-metrics update <session_id> <url>...          PR URL を追加
hitl-metrics update --mark-checked <session_id>... backfill_checked をセット
hitl-metrics update --by-branch <repo> <branch> <url>  ブランチ全セッションに URL 追加
```

## E2E テスト環境（開発者向け）

テストデータを使った Grafana 環境を Docker で起動できます。

```fish
make grafana-up          # Grafana + Image Renderer 起動 → http://localhost:13000
make grafana-screenshot  # 全パネルの PNG を取得
make grafana-down        # コンテナ停止
```

## トラブルシューティング

### hook が動作しない

- `~/.claude/settings.json` に `hitl-metrics hook <event>` が登録されているか確認
- `hitl-metrics` コマンドが PATH に存在するか確認: `which hitl-metrics`
- デバッグログを確認: `~/.claude/logs/session-index-debug.log`

### sync-db でデータが空になる

- `~/.claude/session-index.jsonl` が存在し、データが記録されているか確認
- `~/.claude/logs/permission.log` が存在するか確認

### Grafana でデータが表示されない

- データソースの Path が `hitl-metrics.db` のフルパスを指しているか確認
- `hitl-metrics sync-db` を再実行して DB を最新化
- Grafana のデータソース設定で「Test」ボタンを押して接続を確認
