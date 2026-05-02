# ADR-019: backfill を launchd 定期バッチから Stop hook に移行する

## ステータス

採用済み

## 関連 ADR

- 依存: ADR-006（本 ADR で置換する対象。launchd/cron バッチ方式の決定）
- 依存: ADR-005（Stop フック backfill の初期設計。ADR-006 で部分廃止済み）
- 関連: ADR-010（並列実行化。セマフォ制御・backfill_checked フラグは引き続き有効）

## コンテキスト

ADR-006 で Stop フックの fire-and-forget 方式から launchd/cron 定期バッチに移行した。当時の判断根拠は以下の3点だった。

1. **過去分が拾えない**: Stop フックは当該セッションしか処理せず、ADR-005 以前のセッションや Stop フック未発火セッションの `pr_urls` が空のまま残る
2. **重複 API 呼び出し**: 同一 branch の複数セッションごとに `gh pr view` が走る
3. **Stop フックの複雑性**: session-index.jsonl の読み込み・Popen 起動ロジックが Stop フックに混入

しかし ADR-006 以降の変遷で前提が変わった。

- **Go CLI への集約**: backfill ロジックは `hitl-metrics backfill` コマンドに集約済み。Stop フックは `hitl-metrics backfill; hitl-metrics sync-db` を呼ぶだけでよく、複雑なロジックは混入しない
- **冪等処理の基盤**: `backfill_checked` フラグ（ADR-010）と `is_merged` フラグにより、何回実行しても安全な設計が既にある
- **repo+branch グルーピング**: `backfill.Run()` が重複排除済みのため、セッション数分の API 呼び出しは発生しない

一方、launchd 登録は hitl-metrics セットアップにおける**唯一の Claude Code 外手作業**であり、UX 上のボトルネックになっている。hooks は `.claude/settings.json` で宣言的に管理でき、導入障壁がゼロである。

## 設計案

### 案A: Stop hook + cursor ベースの冪等 backfill（採用）

Stop フック（`hooks/stop.sh`）から `hitl-metrics backfill && hitl-metrics sync-db` を実行する。処理の効率化のため cursor（watermark）を導入し、前回処理済み位置以降のエントリのみを対象にする。

**cursor の設計:**

- `~/.claude/hitl-metrics-state.json` に以下を保存:
  - `last_backfill_offset`: session-index.jsonl の前回処理済み行数（Phase 1 用）
  - `last_meta_check`: Phase 2（マージ判定）の最終実行時刻
- Phase 1（URL 補完）: cursor 以降の新規エントリのみ走査。ただし `backfill_checked=false` かつ `pr_urls` が空のものに限定（既存ロジックと同じ）
- Phase 2（マージ判定）: `last_meta_check` から一定間隔（例: 1時間）経過時のみ実行。未マージ PR の State を再チェック

**Stop hook スキップ時の動作:**

cursor はあくまで効率化のヒント。`--recheck` 相当のフルスキャンは `backfill_checked` フラグで制御されるため、cursor が古くても正しく動作する。次回の Stop hook 発火時に未処理分をまとめて補完する。

**launchd 資材の扱い:**

- `configs/launchd/com.user.hitl-metrics-sync.plist` を削除
- `docs/setup.md` から launchd セットアップ手順を削除し、hooks 設定の説明に置換

### 案B: launchd 定期バッチを維持（却下）

却下理由: hooks だけで完結する UX が実現可能な状況で、launchd という外部依存を残す理由がない。毎時バッチより Stop hook の方がデータ鮮度も高い。

### 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `hooks/stop.sh`（新規） | Stop hook エントリポイント。`hitl-metrics backfill && hitl-metrics sync-db` を実行 |
| `internal/backfill/backfill.go` | cursor 読み込み・書き込みロジックを追加。Phase 2 の時間間隔チェック |
| `internal/backfill/state.go`（新規） | `hitl-metrics-state.json` の読み書き |
| `configs/launchd/com.user.hitl-metrics-sync.plist` | 削除 |
| `docs/setup.md` | launchd セットアップ手順を削除、hooks 設定に置換 |
| `.claude/settings.json` への hooks 追加案内 | Stop hook の登録方法をドキュメント化 |
