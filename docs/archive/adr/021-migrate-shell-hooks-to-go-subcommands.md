# ADR-021: hooks の Shell スクリプトを Go サブコマンドに統一する

## ステータス

採用済み

## 関連 ADR

- 関連: ADR-001（session-index.sh の設計）
- 関連: ADR-003（permission UI 計測の基盤）
- 関連: ADR-014（PermissionRequest フックへの移行）
- 関連: ADR-019（Stop hook での backfill 実行）

## コンテキスト

データ収集層の hooks は現在5つの Shell スクリプトで実装されている。

| スクリプト | hook イベント | 行数 |
|---|---|---|
| `session-index.sh` | SessionStart | ~30 |
| `permission-log.sh` | PermissionRequest | ~35 |
| `pretooluse-track.sh` | PreToolUse | ~30 |
| `stop.sh` | Stop | ~3 |
| `todo-cleanup-check.sh` | SessionStart | ~80 |

以下の課題がある：

1. **ロジック重複**: `permission-log.sh` と `pretooluse-track.sh` で tool name アノテーション（internal/external 判定、Bash コマンド抽出）が完全に重複している
2. **保守性**: `todo-cleanup-check.sh` は80行の embedded awk でテスト不能
3. **配布の二重構造**: Go バイナリに Shell スクリプトを `//go:embed` で埋め込み → `ExtractHooks()` でディスクに展開 → `install` で `settings.json` に登録、という3段階が必要
4. **言語の混在**: データ変換層（Go CLI）とデータ収集層（Shell）で実装言語が異なり、JSON パースや git 操作のロジックが共有できない

## 設計案

### 案A: 全 Go 化 — `hitl-metrics hook <event>` サブコマンド（採用）

全 hook を `hitl-metrics hook <event-name>` サブコマンドとして Go で実装する。

```
hitl-metrics hook session-start       # session-index.sh + todo-cleanup-check.sh
hitl-metrics hook permission-request  # permission-log.sh
hitl-metrics hook pre-tool-use        # pretooluse-track.sh
hitl-metrics hook stop                # stop.sh（backfill + sync-db 呼び出し）
```

`settings.json` の hook 登録は以下のように変わる：

```json
// Before
{ "type": "command", "command": "~/.local/share/hitl-metrics/hooks/session-index.sh" }

// After
{ "type": "command", "command": "hitl-metrics hook session-start" }
```

**メリット**:
- Shell スクリプトの埋め込み・展開・パス管理が不要になる（`embed.go`、`ExtractHooks()` を削除）
- tool annotation ロジックを `internal/hook/` パッケージの共通関数に統合できる
- `todo-cleanup-check` の awk パースを Go で書き直し、テスト可能にできる
- `hitl-metrics install` は PATH 上のバイナリを前提とするだけで済む

**デメリット**:
- Go バイナリの起動コスト（~10ms）が hook ごとに発生する。ただし hook は人間の操作間隔（秒単位）で発火するため体感影響なし
- 既存ユーザーは `hitl-metrics install` の再実行が必要

### 案B: Shell 維持（却下）

現状維持。重複ロジックと awk の保守性問題が残り続ける。

### 案C: 一部のみ Go 化（却下）

複雑な hook（todo-cleanup-check、permission-log + pretooluse-track）のみ Go 化する。配布の二重構造（Go サブコマンド + Shell スクリプト展開）が残り、install コマンドの複雑性が増す。

### 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `internal/hook/` (新規) | hook サブコマンドのハンドラ実装（session-start, permission-request, pre-tool-use, stop） |
| `cmd/hitl-metrics/main.go` | `hook` サブコマンドのルーティング追加 |
| `internal/install/install.go` | `hookDefs` のコマンドを Go サブコマンドに変更 |
| `internal/install/embed.go` | 削除（Shell スクリプト埋め込み不要） |
| `internal/install/hooks/*.sh` | 削除 |
| `hooks/*.sh` | 削除 |
| `docs/architecture.md` | 「hooks（Shell）」→「hooks（Go）」に更新 |
