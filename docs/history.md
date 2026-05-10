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

## ADR 索引

過去の意思決定 23 件を `docs/archive/adr/` に保存している。

| # | ステータス | 領域 | タイトル |
|:---:|:---:|:---|:---|
| 001 | 採用済み | hooks | Claude セッションを PR ベースで追跡する |
| 002 | 採用済み | hooks | SessionStart の `gh pr view` 削除による起動最適化 |
| 003 | 廃止 (ADR-023) | hooks | Notification hook による permission UI 計測 |
| 004 | Superseded (ADR-007) | metrics | 作業量で正規化した自律度指標 |
| 005 | 部分廃止 (ADR-019) | hooks | Stop hook で既存 PR URL を補完 |
| 006 | 廃止 (ADR-019) | batch | launchd / cron バッチによる backfill |
| 007 | 廃止 (ADR-018) | metrics | 人の介入指標の拡張（perm_rate, mid_session_msgs 等） |
| 008 | 廃止 (ADR-023) | dashboard | perm rate 時系列トレンドグラフ |
| 009 | 廃止 (ADR-023) | hooks | permission UI 内訳の監視 |
| 010 | 採用済み | batch | バックフィルバッチの並列実行化 + `backfill_checked` フラグ |
| 011 | 採用済み | dashboard | ゴーストセッションをセッション数カウントから除外 |
| 012 | 廃止 (ADR-023) | dashboard | ツール別 permission UI テーブル |
| 013 | 部分廃止 | 複合 | hitl-metrics のディレクトリ隔離 → 別リポジトリへ |
| 014 | 廃止 (ADR-023) | hooks | permission-log を PermissionRequest hook へ移行 |
| 015 | 採用済み | dashboard | SQLite + Grafana を可視化基盤に選定 |
| 016 | 採用済み | e2e | Grafana Image Renderer による E2E スクリーンショット |
| 017 | 採用済み | workflow | 設計/実装セッション分離の自動 dispatch |
| 018 | 採用済み | metrics | merged PR スコープ・タスク種別分類への再設計 |
| 019 | 採用済み | hooks | backfill を launchd から Stop hook に再移行 |
| 020 | Draft（凍結→ADR-023 で実現） | metrics | transcript_stats への token 使用量カラム追加 |
| 021 | 採用済み | hooks | hooks の Shell スクリプトを Go サブコマンドに統一 |
| 022 | 一部廃止 (ADR-023) | dashboard | ダッシュボードのアクショナビリティ改善 |
| 023 | 採用済み | metrics | PR 単位のトークン消費効率メトリクスを導入 |

---

## 残っている有効な決定の要点

`docs/design.md` の散文では細かい根拠まで踏み込まないため、以下を index として残す。

| 決定 | 理由 | 参照 |
|---|---|---|
| `pr_urls` を JSON Lines で増分更新 | hook 実行中に DB を触らないため。重複排除しつつ追記のみで競合回避 | ADR-001 |
| `(repo, branch)` グルーピング + `backfill_checked` で永続スキップ | PR を作らないブランチ（main / master）の毎時 8 秒空振りを排除 | ADR-010 |
| `is_ghost` 判定で空セッションを除外 | Claude Code の `file-history-snapshot` UUID JSONL が SessionStart で誤記録される問題への対処 | ADR-011 |
| `pr_metrics` を merged PR スコープに限定 | 未マージ・放棄 PR のノイズを排除し、最終成果物のみを評価対象とする | ADR-018 |
| `transcript_stats` に token usage を保存 | `pr_metrics` の主指標を「PR あたりのトークン消費効率」に切り替えるため | ADR-023 |
| permission_events テーブルを廃止 | Claude Code の auto mode 進化で permission UI 計測の長期価値が低下したため | ADR-023 |
| `task_type` を集計軸から廃止（schema には残す） | branch 命名規約への依存・命名と内容の乖離・「同種タスクは複雑度が近い」という暗黙前提の崩壊。定性評価は LLM 評価層に委ねる方針 | ADR-024 |
| `install` で hook を自動登録しない | dotfiles 等で settings.json を一元管理する構成と整合させるため | （ADR なし、運用判断） |
| `doctor` で hook を自動修復しない | 同上 | （ADR なし、運用判断） |

---

## 廃止された設計と理由

### permission UI 計測の系譜

`Notification (permission_prompt)` hook → 発火不安定 → `PermissionRequest` hook（ADR-014）で安定化 → ADR-023 で系統ごと廃止。

permission UI の発生率を `perm_rate = perm_count / tool_use_total` で正規化する設計（ADR-007）も含めて、現在は `pr_metrics` から完全に削除されている。`hitl-metrics install` は PermissionRequest / PreToolUse hook を新規登録しない。

既存ユーザーの `~/.claude/settings.json` に登録済みの旧 hook の自動削除は行わない。`hitl-metrics install --uninstall-hooks` で旧 install が書き込んだ単一エントリのみ削除可能。

### launchd / cron 定期バッチ

ADR-006 で導入した `session-index-backfill-batch.py` は launchd `StartCalendarInterval` で毎時実行していた。Go CLI への集約と cursor 設計の確立後、Claude Code 外の唯一の手作業を残す UX 上の理由がなくなり ADR-019 で Stop hook + cursor 方式に置き換えた。

### 設計/実装セッション分離の自動 dispatch

ADR-017 で導入した `/dispatch` skill は user skill 化したため、本リポジトリの `.claude/skills/dispatch/` は削除済み。TODO.md の実装可能タスクを入力ソースとして worktree + tmux で並列ディスパッチするフロー自体は `impl` skill が引き継いでいる（入力ソースは `実装タスク` セクションに統一済み）。
