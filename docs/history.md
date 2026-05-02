# hitl-metrics 経緯

この文書は過去の実装・判断の背景を記録する。
現在の外部契約は `docs/spec.md`、実装設計は `docs/design.md` を参照する。

過去の意思決定の詳細は `docs/archive/adr/` 配下に保存している（旧 ADR フォーマット）。
新規の設計判断は ADR を作らず、`docs/design.md` の更新と Contextual Commits で記録する。

---

## 大きな方針転換

### 1. dotfiles 内ディレクトリ → 別リポジトリ分離（2026-03-09）

当初は dotfiles の `configs/claude/scripts/` 配下に hook スクリプトが他の Claude Code スクリプトと混在していた。`hitl-metrics/` ディレクトリへの集約で結合度を下げ、その後 dotfiles ADR の 14 件を占有していた問題と開発プロセスの完全独立を理由に、別リポジトリへ完全分離した。
[ADR-013](archive/adr/013-claude-stats-directory-isolation.md)

### 2. 純 Python ダッシュボード → SQLite + Grafana

`permission-ui-server.py` が集計と表示を 1 ファイルに混載していた。Prometheus を検討したが「任意の日付範囲で PR 別集計」という用途と合わず、SQLite + Grafana を選定。Grafana の SQLite データソースで JSONL → SQLite の単純変換だけで任意集計が可能になった。
[ADR-015](archive/adr/015-dashboard-visualization-backend-selection.md)

### 3. Shell hooks → Go サブコマンド統一

`session-index.sh` / `permission-log.sh` / `pretooluse-track.sh` / `stop.sh` / `todo-cleanup-check.sh` の 5 本を `hitl-metrics hook <event>` の Go サブコマンドに統合した。これにより配布の二重構造（Go バイナリ + Shell embed + 展開）が解消され、tool annotation などのロジックが共通化された。
[ADR-021](archive/adr/021-migrate-shell-hooks-to-go-subcommands.md)

### 4. permission UI 計測中心 → PR トークン効率中心

主指標は `perm_rate`（permission UI 発生率）から `total_tokens` / `pr_per_million_tokens` に切り替えた。Claude Code の auto mode の進化により permission UI は構造的に減少していくため、長期的な改善対象としてトークン効率の方が安定していると判断した。これに伴い permission_events テーブル・PermissionRequest hook・関連 Grafana パネルをすべて廃止した。
[ADR-023](archive/adr/023-pr-token-efficiency-metrics.md)

### 5. Claude Code 単一エージェント → マルチコーディングエージェント対応（2026-05-02）

当初は Claude Code 専用ツールとして設計されていたため、データディレクトリ (`~/.claude/`)・hook 入力スキーマ・transcript 形式が暗黙にハードコードされていた。Codex CLI が lifecycle hooks（SessionStart / Stop / PostToolUse 等）と rollout JSONL（`event_msg.payload.type=="token_count"` で累積トークン記録）を提供しはじめたため、両エージェントを単一の SQLite DB に集約する構成に拡張した。

主な設計判断:

- **DB は単一に集約** — `~/.claude/hitl-metrics.db` を維持し、`sessions.coding_agent` カラムで claude/codex を区別する。Grafana datasource 設定の後方互換を壊さないため。
- **データ収集元は agent ごとに分離** — `~/.claude/session-index.jsonl` と `~/.codex/session-index.jsonl` を別ファイルにし、`internal/agent/{claude,codex}/` のアダプタで読み込み元と transcript パーサだけを差し替える。
- **PRIMARY KEY を複合化** — `(session_id, coding_agent)`。両 agent の UUID 衝突に対する保険。
- **Codex の SessionEnd 不在を Stop hook で代替** — Stop が応答完了ごとに発火するため `ended_at` を毎回上書き。プロセス kill のケースは backfill が rollout JSONL の最終 event タイムスタンプで補正する。
- **`reasoning_tokens` カラム追加** — Codex の token_count イベントに reasoning が独立して記録されるため、Claude では 0 固定、Codex では実値を入れる。
- **`--agent` フラグ既定値を `claude` に** — 既存の `~/.claude/settings.json` 登録（フラグなし）が壊れないようにするため。
- **`install` → `setup` リネーム + `uninstall-hooks` 独立化** — 旧 `install` は何も書き込まないのに `--uninstall-hooks` だけが破壊的、という非対称が認知的負荷だった。マルチエージェント対応で `--agent codex` が増えると「install というコマンドが Codex 設定に書き込みに来るのでは」という誤解を一層誘発する見通しがあったため、CLI 表面が変わる節目にまとめてリネームした。旧 `install` は deprecation warning 付き alias として残す。

### 6. CHANGELOG.md → history.md / GitHub Release への一本化（2026-05-02）

ADR から spec/design/history へ移行した直後は CHANGELOG.md と history.md が並走していた。両者は WHAT（日付ごとの変更）と WHY（方針転換の経緯）として理屈の上では役割が分離していたが、実態としては ADR 番号参照や同一イベントの重複記載で境界が緩んでいた。goreleaser によるバイナリ配布で GitHub Release が事実上のリリースノートとして機能していたため、CHANGELOG.md を廃止して以下に再分配した。

- リリース単位の WHAT → GitHub Release（タグ push 時に goreleaser が生成）
- 方針転換の WHY → `docs/history.md`
- 1 コミット内の判断 → Contextual Commits のアクション行
- 個別コミットの WHAT → `git log`

これに伴い `todo-cleanup` hook の「TODO.md 完了タスク → CHANGELOG.md 移送」動作は「TODO.md からの削除のみ」に変更する（`git-ship` skill の CHANGELOG チェックステップも削除）。

### 7. backfill 方式の変遷

| 段階 | 方式 | 廃止理由 |
|---|---|---|
| 初期 (ADR-005) | Stop hook で fire-and-forget で `gh pr view` | 過去分が拾えない、重複 API 呼び出し |
| 中期 (ADR-006) | launchd / cron 定期バッチ | Claude Code 外の唯一の手作業で UX が悪化 |
| 現在 (ADR-019) | Stop hook + cursor + Go CLI 集約 | Go CLI への集約で複雑性問題が解消、cursor で増分処理 |

[ADR-005](archive/adr/005-session-index-pr-url-backfill-on-stop.md) → [ADR-006](archive/adr/006-session-index-pr-url-backfill-cron-batch.md) → [ADR-019](archive/adr/019-backfill-stop-hook-migration.md)

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
