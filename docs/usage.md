# 使い方・動作説明

hitl-metrics の動作の仕組み、日常の運用、トラブルシューティングについて説明します。セットアップ手順は [setup.md](setup.md) を参照してください。

## データフロー

```
Claude Code hooks → ~/.claude/session-index.jsonl + transcript JSONL ┐
                                                                     ├→ hitl-metrics backfill / sync-db
Codex CLI hooks   → ~/.codex/session-index.jsonl  + rollout JSONL    ┘
                                                                     → ~/.claude/hitl-metrics.db (SQLite)
                                                                     → Grafana
```

DB は両 agent を集約して `~/.claude/hitl-metrics.db` に書き出されます。`sessions.coding_agent` カラムで `claude` / `codex` を区別します。

## hook の役割

| hook | 対象 agent | トリガー | 出力先 |
|------|-----------|----------|--------|
| `hitl-metrics hook session-start --agent claude` | Claude | セッション開始時 | `~/.claude/session-index.jsonl` |
| `hitl-metrics hook session-end --agent claude` | Claude | セッション終了時 | `~/.claude/session-index.jsonl`（`ended_at`, `end_reason`）+ SQLite 同期 |
| `hitl-metrics hook stop --agent claude` | Claude | 応答完了時 | `~/.claude/hitl-metrics.db`（backfill + sync-db） |
| `hitl-metrics hook session-start --agent codex` | Codex | セッション開始時 | `~/.codex/session-index.jsonl` |
| `hitl-metrics hook stop --agent codex` | Codex | 応答完了時 | `~/.codex/session-index.jsonl`（`ended_at` 上書き）+ backfill + sync-db |
| `hitl-metrics hook post-tool-use --agent codex` | Codex | tool 実行直後 | `~/.codex/session-index.jsonl`（`pr_urls` 追記） |
| `hitl-metrics hook todo-cleanup` | Claude | セッション開始時（main） | `TODO.md`（完了タスクを削除） |

Codex には `SessionEnd` イベントが存在しないため、`Stop` hook 発火ごとに `ended_at` を上書きし、最後の `Stop` 発火が事実上の SessionEnd になります。プロセスが kill された場合は `backfill` フェーズで rollout JSONL の最終 event タイムスタンプから `ended_at` を補完します。

## agent の自動切替

hook サブコマンド・CLI コマンドで `--agent <claude|codex>` を指定できます。省略時の優先順位は次の通り:

1. `--agent` フラグ
2. 環境変数 `HITL_METRICS_AGENT`
3. `~/.claude/session-index.jsonl` / `~/.codex/session-index.jsonl` の存在に基づく自動検出
4. それでも決まらない場合は `claude` を既定値とする

`backfill` / `sync-db` / `doctor` は検出された agent **すべて** を対象にします。明示的に絞り込みたいときだけ `--agent` を指定してください。

## データ同期の仕組み

Stop hook がセッション終了時に `hitl-metrics backfill --agent <name>` → `hitl-metrics sync-db` を実行します。

- **backfill Phase 1（URL 補完）**: `pr_urls` が空のセッションに対し、`gh pr list` で PR URL を取得
- **backfill Phase 2（マージ判定）**: 未マージ PR の `is_merged` と `review_comments` を更新（1時間間隔）
- **backfill Codex `ended_at` 補完**: rollout JSONL の最終 event タイムスタンプを `ended_at` に反映（hook 未実行のケース対策）
- **sync-db**: 両 agent の JSONL/transcript を読み、`~/.claude/hitl-metrics.db` を毎回 DROP & CREATE で再構築。`sessions.coding_agent` で agent を区別

cursor（`~/.claude/hitl-metrics-state.json` / `~/.codex/hitl-metrics-state.json`）により前回処理済み以降のエントリのみが走査されるため、高速に完了します。

## 日常の運用

Stop hook が登録済みであれば、セッション終了時に `backfill` と `sync-db` が自動実行されます。手動で即時更新する場合:

```fish
hitl-metrics backfill        # 検出された agent すべて
hitl-metrics sync-db         # 検出された agent すべて
hitl-metrics backfill --agent codex
```

ダッシュボードは `http://localhost:3000`（デフォルト）でアクセスできます。

## CLI コマンド

```
hitl-metrics setup [--agent <claude|codex>]            セットアップ案内を表示（hook 登録は dotfiles または手動）
hitl-metrics uninstall-hooks                           旧 install で登録された hook を ~/.claude/settings.json から削除
hitl-metrics doctor                                    検出された agent ごとに binary / data dir / hook 登録を検証
hitl-metrics backfill [--recheck] [--agent <a>]        PR URL の一括補完
hitl-metrics sync-db [--agent <a>]                     JSONL/transcript → SQLite 変換
hitl-metrics update <session_id> <url>...              session-index.jsonl に PR URL を追加
hitl-metrics update --mark-checked <session_id>...     backfill_checked をセット
hitl-metrics update --by-branch <repo> <branch> <url>  ブランチ全セッションに URL 追加
hitl-metrics install                                   廃止予定 alias。setup へ誘導する warning を出す
hitl-metrics version                                   version を表示
```

## Grafana を Docker で起動する

実データ（`~/.claude/hitl-metrics.db`）を使った Grafana 環境:

```fish
make grafana-up          # 実 DB を mount して Grafana + Image Renderer 起動 → http://localhost:13000
make grafana-down        # コンテナ停止
```

DB パスを上書きしたい場合は `HITL_METRICS_DB` を渡す:

```fish
make grafana-up HITL_METRICS_DB=/custom/path/hitl-metrics.db
```

> **注意**: mount は読み書き可能です（SQLite が WAL モードのため `:ro` mount は不可）。frser-sqlite-datasource は SELECT のみで書き込みは行わないので実害はありませんが、Grafana コンテナに DB ファイルへの書き込み権限が渡る点を留意してください。

### E2E テスト環境（開発者向け）

決定的な fixture データを使った Grafana 環境（README 用スクリーンショット生成に使用）:

```fish
make grafana-up-e2e      # fixture 生成 → Grafana 起動
make grafana-screenshot  # 全パネルの PNG を取得
make grafana-down        # コンテナ停止
```

## トラブルシューティング

### hook が動作しない

- `hitl-metrics doctor` で binary / data dir / hook 登録状況を agent ごとに一括確認
- 未登録の hook があれば dotfiles または手動で `~/.claude/settings.json` または `~/.codex/hooks.json` に追加（[setup.md](setup.md#2-hook-の登録) 参照）
- デバッグログを確認: `~/.claude/logs/session-index-debug.log` または `~/.codex/logs/session-index-debug.log`

### sync-db でデータが空になる

- `~/.claude/session-index.jsonl` または `~/.codex/session-index.jsonl` が存在し、データが記録されているか確認
- session-index の `transcript` パスが存在するか確認
- Codex の場合、`.jsonl.zst` 圧縮ファイルでも透過解凍されるはずです

### Grafana でデータが表示されない

- データソースの Path が `hitl-metrics.db` のフルパスを指しているか確認
- `hitl-metrics sync-db` を再実行して DB を最新化
- Grafana のデータソース設定で「Test」ボタンを押して接続を確認
- ダッシュボードの `coding_agent` テンプレート変数が `All` になっているか確認
