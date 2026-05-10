# agent-telemetry 経緯

この文書は過去の大方針転換のナラティブ要約を記録する。
現在の外部契約は `docs/spec.md`、実装設計は `docs/design.md` を参照する。

意思決定の primary store は `issues/` 配下（[0011](../issues/closed/0011-feat-structured-intent-on-issues.md) で確立）。各方針転換の詳細な根拠・問題・解決方法・採用しなかった代替は対応する retro issue を参照する（11 件は 2026-05-10 に [0013](../issues/closed/0013-design-history-md-retro-conversion.md) で `issues/closed/0014-...` 〜 `0024-...` に retro 化）。

過去の意思決定の旧フォーマットは `docs/archive/adr/` 配下に保存している（参照のみ）。新規の設計判断は ADR を作らず、`docs/design.md` の更新と Contextual Commits + 必要なら issue で記録する。

---

## 大きな方針転換

### 1. dotfiles 内ディレクトリ → 別リポジトリ分離（2026-03-09）

dotfiles の `configs/claude/scripts/` 配下にあった hook を、`hitl-metrics/` ディレクトリへ集約 → 結合度低下と開発プロセス独立を理由に別リポジトリへ完全分離。

詳細: [issues/closed/0014-process-dotfiles-to-standalone-repo.md](../issues/closed/0014-process-dotfiles-to-standalone-repo.md) ・ [ADR-013](archive/adr/013-claude-stats-directory-isolation.md)

### 2. 純 Python ダッシュボード → SQLite + Grafana

`permission-ui-server.py` の集計 + 表示一体構成を解体。Prometheus は「任意の日付範囲で PR 別集計」と合わず、SQLite + Grafana を採用。JSONL → SQLite の単純変換で任意 SQL 集計が成立。

詳細: [issues/closed/0015-design-sqlite-grafana-as-visualization-backend.md](../issues/closed/0015-design-sqlite-grafana-as-visualization-backend.md) ・ [ADR-015](archive/adr/015-dashboard-visualization-backend-selection.md)

### 3. Shell hooks → Go サブコマンド統一

`session-index.sh` 等 5 本の Shell スクリプトを `agent-telemetry hook <event>` の Go サブコマンドに統合。配布の二重構造（Go バイナリ + Shell embed + 展開）が解消され、tool annotation 等の共通ロジックも DRY 化。

詳細: [issues/closed/0016-design-shell-hooks-to-go-subcommands.md](../issues/closed/0016-design-shell-hooks-to-go-subcommands.md) ・ [ADR-021](archive/adr/021-migrate-shell-hooks-to-go-subcommands.md)

### 4. permission UI 計測中心 → PR トークン効率中心

主指標を `perm_rate` から `total_tokens` / `pr_per_million_tokens` に切り替え。Claude Code の auto mode 進化で permission UI が構造的に減るため、改善対象としてはトークン効率の方が長期的に安定。`permission_events` テーブル・`PermissionRequest` hook・関連 panel を全廃。

詳細: [issues/closed/0017-spec-shift-main-metric-to-pr-token-efficiency.md](../issues/closed/0017-spec-shift-main-metric-to-pr-token-efficiency.md) ・ [ADR-023](archive/adr/023-pr-token-efficiency-metrics.md)

### 5. Claude Code 単一エージェント → マルチコーディングエージェント対応（2026-05-02）

Claude 専用前提でハードコードされていた DB / hook / transcript を、Codex CLI と単一 SQLite に集約する構成へ拡張。`sessions.coding_agent` カラムでの区別と agent ごとの adapter 分離（`internal/agent/{claude,codex}/`）が中核。PRIMARY KEY を `(session_id, coding_agent)` 複合化、`reasoning_tokens` カラム追加、`install` → `setup` リネームも同梱。

詳細: [issues/closed/0018-spec-multi-coding-agent-support.md](../issues/closed/0018-spec-multi-coding-agent-support.md)

### 6. CHANGELOG.md → history.md / GitHub Release への一本化（2026-05-02）

CHANGELOG.md と history.md の WHAT / WHY 役割分離が実態として崩れていたため、CHANGELOG.md を廃止して 4 つの store（GitHub Release / history.md / Contextual Commits / `git log`）に再分配。`todo-cleanup` hook の CHANGELOG 移送動作も削除。

詳細: [issues/closed/0019-process-deprecate-changelog-md.md](../issues/closed/0019-process-deprecate-changelog-md.md)

### 7. backfill 方式の変遷

| 段階 | 方式 | 廃止理由 |
|---|---|---|
| 初期 (ADR-005) | Stop hook で fire-and-forget で `gh pr view` | 過去分が拾えない、重複 API 呼び出し |
| 中期 (ADR-006) | launchd / cron 定期バッチ | Claude Code 外の唯一の手作業で UX が悪化 |
| 現在 (ADR-019) | Stop hook + cursor + Go CLI 集約 | Go CLI への集約で複雑性問題が解消、cursor で増分処理 |

詳細: [issues/closed/0020-design-backfill-evolution-to-stop-hook.md](../issues/closed/0020-design-backfill-evolution-to-stop-hook.md) ・ [ADR-005](archive/adr/005-session-index-pr-url-backfill-on-stop.md) → [ADR-006](archive/adr/006-session-index-pr-url-backfill-cron-batch.md) → [ADR-019](archive/adr/019-backfill-stop-hook-migration.md)

### 8. リポジトリ名変更 — hitl-metrics → agent-telemetry（2026-05-04）

「HITL」が ML / ロボティクス由来で coding agent 領域に焦点が合わないため、実態に即した `agent-telemetry` に改名。個人ツールの特性を活かし、後方互換 shim を残さず破壊的変更を一度で済ませる方針（バイナリ名・モジュールパス・DB ファイル名・環境変数・Grafana UID を一括置換、DB の自動マイグレーションのみ提供）。

詳細: [issues/closed/0021-spec-rename-hitl-metrics-to-agent-telemetry.md](../issues/closed/0021-spec-rename-hitl-metrics-to-agent-telemetry.md)

### 9. PR resolve を Stop hook の early binding に切替（2026-05-07）

`pr_urls` 末尾採用 + late binding モデルが、PostToolUse 正規表現の URL 汚染とブランチ再利用で誤接続を起こしていた。Stop hook で `gh pr list --head <branch> --author @me` を 1 回叩いて pin し、`pr_pinned: true` で session に束縛する early binding に切替。検討した 4 案（A/B/C/D）のうち C を採用。

詳細: [issues/closed/0022-design-pr-resolve-early-binding.md](../issues/closed/0022-design-pr-resolve-early-binding.md) ・ 関連バグ: [0001](../issues/closed/0001-bug-pr-session-misattribution.md)

### 10. TODO.md の廃止 — issues/ への一本化（2026-05-08）

`TODO.md` が CLAUDE.md / AGENTS.md の 3 本柱（spec / design / history）の外側にある orphan になっていたため削除。「実装タスク」を open issue、「検討中」を pending issue にマッピングし、`todo-cleanup` hook も廃止。impl skill の入力ソースは `issues/*.md` の列挙に変更。

詳細: [issues/closed/0023-process-deprecate-todo-md.md](../issues/closed/0023-process-deprecate-todo-md.md)

### 11. user 識別子の導入 — マルチユーザー集約への布石（2026-05-08）

サーバ集約構成（[0009](../issues/0009-feat-server-side-metrics-pipeline.md)）に向けて、`session-index.jsonl` と `sessions` テーブルに `user_id` を導入。取得順序は env → TOML → `git config --global user.email` → `unknown`。`git config --local` は cwd 依存で人物が分裂するため意図的に見ない。`pr_metrics` の GROUP BY に `user_id` を追加し pair coding に対応。

詳細: [issues/closed/0024-spec-introduce-user-id-field.md](../issues/closed/0024-spec-introduce-user-id-field.md)

---

## 索引

- ADR 23 件: [docs/archive/adr/index.md](archive/adr/index.md)（旧フォーマット、参照のみ）
- 現在も生きている設計判断（`pr_urls` 採用ルール / `is_ghost` / `(repo, branch)` グルーピング / merged PR スコープ / `task_type` 集計軸廃止 等）: [docs/design.md](design.md)
- retro 化された個別決定: `issues/closed/0014-...` 〜 `0026-...`（[0013](../issues/closed/0013-design-history-md-retro-conversion.md) で 11 件、`install`/`doctor` 運用判断と `/dispatch` skill 移管を [0025](../issues/closed/0025-process-no-auto-hook-registration.md) / [0026](../issues/closed/0026-process-dispatch-skill-moved-to-user-level.md) で追加 retro 化）
