# ADR-021: hooks の Shell スクリプトを Go サブコマンドに統一する

- ステータス: 採用済み
- 領域: hooks / CLI
- 日付: 2026-03-29

## コンテキスト

hitl-metrics の hook は Shell スクリプト（Bash）で実装されていた。Go CLI バイナリに `go:embed` で Shell スクリプトを同梱し、`hitl-metrics install` で展開する方式を取っていた。

この方式には以下の課題があった:

1. **二重メンテナンス**: ビジネスロジック（ツールアノテーション、TODO パース等）が Shell と Go に分散
2. **テスト困難**: Shell スクリプトの awk 処理（todo-cleanup-check.sh）にユニットテストが書けない
3. **配布の複雑さ**: バイナリに Shell スクリプトを embed し、install 時にファイルシステムに展開する二段階が必要
4. **ロジック重複**: permission-log.sh と pretooluse-track.sh が同一のツールアノテーション処理を独立実装

## 決定

全 hook を `hitl-metrics hook <event-name>` Go サブコマンドとして実装する。

### hook サブコマンド

| サブコマンド | イベント | 旧スクリプト |
|---|---|---|
| `hook session-start` | SessionStart | session-index.sh |
| `hook permission-request` | PermissionRequest | permission-log.sh |
| `hook pre-tool-use` | PreToolUse | pretooluse-track.sh |
| `hook stop` | Stop | stop.sh |
| `hook todo-cleanup` | SessionStart | todo-cleanup-check.sh |

### 主要な設計判断

1. **ツールアノテーション共通化**: `AnnotateTool()` 関数で Bash/Read/Write/Edit/Grep の internal/external 分類を統一
2. **install の簡素化**: `go:embed` + ファイル展開を廃止し、`hitl-metrics hook <event>` コマンド文字列を直接 settings.json に登録
3. **stop hook は os/exec**: sqlite 依存を hook パッケージに持ち込まないよう、`hitl-metrics backfill` と `hitl-metrics sync-db` を外部コマンドとして呼び出す

## 結果

### 変更されるファイル

- **新規**: `internal/hook/` パッケージ（input.go, annotate.go, sessionstart.go, permissionrequest.go, pretooluse.go, stop.go, todocleanup.go + テスト）
- **変更**: `cmd/hitl-metrics/main.go`（hook サブコマンド追加）、`internal/install/install.go`（Go サブコマンド形式に変更）
- **削除**: `hooks/*.sh`、`internal/install/embed.go`、`internal/install/hooks/`
- **新規**: `docs/architecture.md`

### メリット

- Shell スクリプト廃止により Go 単一バイナリで完結
- AnnotateTool のロジック重複が解消
- todo-cleanup の TODO パース処理に Go テストが追加
- install が `--hooks-dir` 不要になりシンプル化

### 注意点

- 既存の Shell hook が settings.json に登録済みの環境では、`hitl-metrics install` を再実行して新形式のエントリを追加する必要がある（旧エントリは手動削除）
