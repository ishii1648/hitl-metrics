# TODO

hitl-metrics の開発タスクを管理する。完了したタスクは削除する。変更履歴は git log と GitHub Release を参照し、設計の経緯は `docs/history.md` に集約する。

## 実装タスク

- Codex CLI 対応 — Grafana ダッシュボード
  - [ ] Agent 別比較 stat パネルを追加（avg tokens / PR と PR / 1M tokens）
  - [ ] `make grafana-screenshot` を実行して `docs/images/dashboard-*.png` を更新する

- リポジトリリネーム — agent-telemetry
  - 背景・決定事項（D1〜D4）は `docs/history.md` 「8. リポジトリ名変更 — hitl-metrics → agent-telemetry（2026-05-04）」を参照
  - 本タスクは複数 PR に分割する（フェーズ単位で 1 PR 目安）。各フェーズの受け入れ条件は粒度粗め — 着手時に詳細化する
  - フェーズ 1: GitHub repo リネーム + ローカル ghq 移行
    - [ ] GitHub UI で `hitl-metrics` → `agent-telemetry` にリネーム（自動リダイレクトには依存せず後続フェーズで全 import を置換する前提）
    - [ ] ローカル ghq path を更新: `~/ghq/github.com/ishii1648/hitl-metrics` → `agent-telemetry`
    - [ ] 進行中の worktree / feature ブランチを新パス配下に移行
  - フェーズ 2: コードベース置換
    - [ ] `go.mod` の module path を `github.com/ishii1648/agent-telemetry` に変更
    - [ ] import 文 `github.com/ishii1648/hitl-metrics` → `github.com/ishii1648/agent-telemetry` を全置換
    - [ ] `cmd/hitl-metrics/` を `cmd/agent-telemetry/` にリネーム
    - [ ] DB ファイル名のデフォルトを `hitl-metrics.db` → `agent-telemetry.db` に変更
    - [ ] `internal/setup/` の hook 登録 binary 名を更新
    - [ ] エラーメッセージ・ログ文字列の `hitl-metrics` 言及を置換（環境変数 `HITL_METRICS_AGENT` → `AGENT_TELEMETRY_AGENT` の扱いも併せて決める）
    - [ ] テスト fixture / golden file の参照を更新
    - [ ] メトリクス名プレフィックス `hitl_*` → `agent_*`（D1）— 注: 現状の SQL VIEW / カラム名は `pr_metrics`, `session_concurrency_*` などプレフィックスなし。該当する `hitl_` 接頭辞のメトリクスが実装済みでないため、本項目は新規メトリクス追加時の命名規約として運用する
  - フェーズ 3: 設定ファイル
    - [ ] `grafana/dashboards/hitl-metrics.json` をリネーム + title / uid / 全 SQL クエリ内のテーブル参照を更新
    - [ ] `grafana/provisioning/{dashboards,datasources}/hitl-metrics*.yaml` をリネーム + 内部参照を更新
    - [ ] `docker-compose.yaml` のサービス名・ボリュームパスを更新
    - [ ] `.goreleaser.yaml` の成果物名を更新
    - [ ] `Makefile` ターゲット内の参照を確認
  - フェーズ 4: ドキュメント全置換
    - [ ] `README.md` を全置換
    - [ ] `AGENTS.md` を全置換
    - [ ] `docs/{spec,design,setup,usage}.md` の残存 `hitl-metrics` 言及を置換
    - [ ] `docs/archive/adr/index.md` の説明文を更新（個別 ADR 本文は歴史記録なので不変）
  - フェーズ 5: マイグレーション・既存環境互換
    - [ ] `~/.claude/hitl-metrics.db` → `~/.claude/agent-telemetry.db` の自動移行スクリプト（`scripts/migrate-db-name.sh` を参考）
    - [ ] `~/.claude/settings.json` / `~/.codex/config.toml` の hooks 設定移行ガイドを `docs/setup.md` に追記
    - [ ] `agent-telemetry doctor` で旧ファイル（旧 DB / 旧 hook 登録）を検出して案内する diagnostic を追加
    - [ ] `agent-telemetry self-upgrade` で旧バイナリ → 新バイナリ移行ハンドリング（旧 `hitl-metrics` バイナリの除去・PATH 案内）
    - [ ] リリースノートに **BREAKING CHANGE** を明記
  - フェーズ 6: 検証 + リリース
    - [ ] `make grafana-screenshot` で `docs/images/dashboard-*.png` を再生成
    - [ ] `make grafana-up` で実データ起動を確認
    - [ ] tag push で GoReleaser 動作確認 + リリース

## 検討中

- Stop hook の `hitl-metrics` PATH 依存をなくす — 解決方針を決める
  - 候補 A: `backfill` / `sync-db` を `internal/` 関数として直接呼ぶ（同一プロセス、PATH 非依存）
  - 候補 B: `setup` 時に hook コマンドの絶対パスを案内する（`settings.json` / `config.toml` 側で絶対パスを書く）
  - 候補 C: hook 内で binary を `os.Executable()` で解決し PATH にフォールバックしない
  - 失敗時ログの設計（PATH 不在 / 内部エラーの切り分け）も方針に含める
  - 方針確定後に受け入れ条件を整えて実装タスクへ昇格させる

- ローカル検証環境と CI の再現性 — 完了条件を具体化する
  - SQLite テストの不安定要因（macOS arm64 で `modernc.org/sqlite` 使用時に発生する事象）を特定し、安定化の具体条件を決める
  - `go test -race` がローカルで実行不能な事例を整理し、代替手順（CI に委ねる / Docker で回す等）の方針を決める
  - 制約整理の記録先（`docs/setup.md` か別 docs か）を決める

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` hook で Bash コマンドの stdout サイズを記録する想定
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定する
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
  - 受け入れ条件（記録先・閾値・集計方法）が未確定

- retro-pr との連携
  - PR の下位・上位 10% ずつは自動で retro-pr 実行する想定
  - 結果を PR と関連付けて表示する想定
  - 受け入れ条件（連携方式・表示先・自動化対象）が未確定
