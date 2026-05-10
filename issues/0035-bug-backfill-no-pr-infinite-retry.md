---
decision_type: design
affected_paths:
  - internal/backfill/backfill.go
  - internal/sessionindex/update.go
  - internal/hook/stop.go
  - docs/design.md
  - docs/spec.md
tags: [backfill, hooks, stop-hook-cost, doc-code-drift]
---

# backfill が PR 未作成ブランチを毎 Stop で無限再試行する

Created: 2026-05-11

## 概要

`internal/backfill/backfill.go` の `fetchPR` (L358-403) は、`gh pr list --head <branch> --author @me --state all --limit 1` が **空** を返したケースで `markChecked` を立てない。これにより:

- `feature/foo` で PR を作らずに worktree が残存 → Stop hook ごとに `gh pr list` を無限に叩く
- `main` / `master` セッション（dotfiles 等）→ こちらも毎 Stop で probe される

`docs/design.md:206` と `docs/spec.md:136` は「PR が存在しないブランチ（`main` / `master` 等）は初回チェック後に `backfill_checked: true` をセットして永続スキップする」と書いているが、code 側はその実装を持っていない（一度も書かれたことがない）。**ドキュメントが先行し、実装が追いついていない doc/code drift**。

加えて、Stop hook の pin lookup と backfill の group probe が **同 tick で同じ `gh pr list` を 2 回叩く**（`docs/design.md:147` 既述）。pin が「PR なし」を確定した直後でも、backfill が独立に再 probe する。

## 根拠

### 経緯（なぜ現状の実装になっているか）

- 原 Python (`batch/session-index-backfill-batch.py`, e1196eb 以前) は **cron 実行前提**で「`cwd` 存在 + PR 未発見 → 次 cron で再試行」をコメント明記の上で意図的に実装していた。cron tick 数（1 日数回）×グループ数のコストは許容範囲だった
- 2026-03 の Go rewrite (e1196eb) はこの挙動を素直に移植
- 2026-04-29 [issues/closed/0020-design-backfill-evolution-to-stop-hook.md](closed/0020-design-backfill-evolution-to-stop-hook.md) で backfill を Stop hook から呼ぶよう移行 → retry 頻度が **cron tick 数 → Stop 回数** に急増
- 同じ流れで `docs/design.md:204-206` に「main/master は永続スキップ」が書かれたが、code 側の `fetchPR` は touch されず Python 起源の挙動が残った
- 2026-05-08 [issues/closed/0022-design-pr-resolve-early-binding.md](closed/0022-design-pr-resolve-early-binding.md) で Stop hook pin が導入され、PR 作成済セッションは pin で即解決するようになった。しかし「PR 未作成ブランチの probe」は backfill フォールバック側に残ったため、本 issue の cost が浮き彫りになった

### 影響

- 個人 repo / dotfiles 利用で `~/.claude/session-index.jsonl` に main/master 系の old session が累積するほど、Stop hook hot path 内の backfill フェーズが線形にコスト増（`(repo, branch)` グループ単位なので最悪 8 並列で頭打ちだが、それでも `gh pr list` 1 回あたり通信遅延が乗る）
- abandoned `feature/*` worktree も同様
- ユーザに見える症状: Stop hook の応答完了後の待ち時間が無駄に伸びる、`gh` API rate limit に余分な圧

## 問題

| シナリオ | 現状 (毎 Stop) | あるべき姿 |
|---|---|---|
| `main` / `master` セッションの累積 | 毎 Stop で `gh pr list` を group probe | 24h 経過後の old session は markChecked、新規分のみ 1 回 probe |
| `feature/foo` で PR 未作成のまま完了 | 同上 | 24h 経過後 markChecked、それ以前は遅延 PR 作成救済のため retry |
| Stop hook の pin が「PR なし」を確定した直後の同 tick | backfill が独立に再 probe（pin と同じ call を 2 回） | 同 tick 内では pin 結果を信用して probe skip |

## 対応方針

採用: **E（`ended_at` ベース horizon）+ G（pin 結果を同 tick 内で再利用）の組み合わせ**。

### E (主軸)

`fetchPR` で `gh pr list` が空を返した group について、group 内の session のうち以下の両方を満たすものを markChecked する:

- `ended_at` が空でない
- `ended_at` から 24h 以上経過している

`ended_at` が空 / 24h 以内のセッションは markChecked しない（次 tick で再 probe = 遅延 PR 作成救済のため）。

これにより:
- main/master のような永続 PR-less group: 古い session は 24h 後に group 一括 markChecked、新規 session のみ 1 tick だけ probe → 自然収束
- abandoned `feature/*`: 24h 後に markChecked
- 完了直後 (~24h 以内) に手動 `gh pr create` する late-binding シナリオ: 引き続き救済

horizon は定数化（例: `MarkCheckedHorizon = 24 * time.Hour`）し、運用感を見て tune 可能にする。

### G (補助、別 PR でも可)

Stop hook の `pinPRForSession` が「PR なし」を確定した `(repo, branch)` を、同 tick 内の backfill フェーズに in-memory で渡し、当該 group の probe を skip する。これで Stop hook hot path の `gh pr list` 重複呼び出しを 1 回減らす。

### 却下した代替案

| 案 | 却下理由 |
|---|---|
| A: empty → 即 markChecked | 24h 以内の遅延 PR 作成を取りこぼす。retry 価値の中で最も大きい時間帯を捨てる |
| B: 試行回数 N | session entry に counter 追加（schema 変更）。`(repo, branch)` group との運用がぎこちない（min(attempts) を取る等の追加ロジック必要）。E に対する利点なし |
| C: `last_checked_at` フィールド追加 | schema 変更のコストに見合わない。`ended_at` の再利用で同等の効果 |
| D: branch 名 `main` / `master` を hardcode 除外 | branch 命名は org / 個人で多様（`trunk` / `dev` / `develop`）。汎用性を損なう。empirical signal + 時間窓のほうが robust |
| F: pin が「PR なし」と判定した時点で session を即 markChecked | pin 直後 ~24h の遅延 PR 作成を救済不可。E と同じ目的を pin 側で（不適切に早く）やってしまう |

## 受け入れ条件

修正された後の振る舞いを以下で検証する:

- [ ] PR 未作成 main セッションは、`ended_at` から 24h 経過後の Stop hook 1 回で当該 session が `backfill_checked = true` になる
- [ ] 24h 以内のセッションは markChecked されず、次 tick で再 probe される（遅延 PR 作成の救済窓を維持）
- [ ] PR 未作成の `feature/foo` セッションでも同様に 24h horizon が効く（branch 名に依存しない）
- [ ] `(repo, branch)` グループ内に新旧 session が混在する場合、24h 経過した session のみが markChecked され、新規 session は markChecked されない（group 全体一括ではなく per-session 判定）
- [ ] Stop hook の pin lookup が「PR なし」を確定した直後の同 tick の backfill では、当該 `(repo, branch)` の `gh pr list` が **追加で呼ばれない**（G 採用時。別 PR に分けるなら別 issue）
- [ ] `agent-telemetry backfill --recheck` は引き続き markChecked を無視してフルスキャンする（既存挙動の回帰なし）
- [ ] `docs/design.md:204-206` の「`(repo, branch)` グルーピングと `backfill_checked`」節を、`ended_at` ベース horizon を反映した記述に更新する
- [ ] `docs/spec.md:136` の `backfill_checked` 説明を、horizon 条件を含む形に更新する
- [ ] 既存の `internal/sessionindex/update_test.go` の `TestMarkChecked_*` を回帰させない
- [ ] `internal/backfill/backfill.go` に horizon 判定の unit test を追加（`ended_at` 空 / 12h 前 / 36h 前 / 混在 group のケース）

## 参照

- 過去の意思決定: [0020](closed/0020-design-backfill-evolution-to-stop-hook.md) (cron→Stop hook 移行) / [0022](closed/0022-design-pr-resolve-early-binding.md) (Stop hook pin 導入) / [0001](closed/0001-bug-pr-session-misattribution.md) (関連バグ)
- 該当コード: `internal/backfill/backfill.go:358-403` (`fetchPR`), `internal/sessionindex/update.go:131-176` (`MarkChecked`)
- doc/code drift 箇所: `docs/design.md:204-206`, `docs/spec.md:136`
- 詳細な調査メモ: `.outputs/claude/backfill-markchecked-investigation.md`（local 出力、commit しない）
