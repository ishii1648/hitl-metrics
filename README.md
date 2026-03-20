# claudedog

Claude Code の人の介入率を追跡・可視化する計測ツール（hook・CLI・ダッシュボード）。

## アーキテクチャ

3層構成:

1. **データ収集層** (`hooks/`) — Claude Code hook で session/permission/tool-use イベントを JSONL/log に記録
2. **データ変換層** (`cmd/claudedog/`, `internal/`) — Go CLI で JSONL/log → SQLite 変換・PR URL 補完
3. **可視化層** (`grafana/`) — Grafana ダッシュボードで介入率・ツール分布を表示

```
hooks → ~/.claude/*.jsonl|log → claudedog sync-db → SQLite → Grafana
```

## 構成

```
hooks/                         # Claude Code hook スクリプト（データ収集層）
├── session-index.sh               # セッション開始時に JSONL へ記録
├── permission-log.sh              # PermissionRequest hook で permission UI をログ
└── pretooluse-track.sh            # PreToolUse hook でツール名を記録
cmd/claudedog/                 # CLI エントリポイント
internal/                      # コアパッケージ
├── sessionindex/                  # session-index.jsonl 読み書き
├── permlog/                       # permission.log パース
├── transcript/                    # transcript JSONL → 統計量抽出
├── syncdb/                        # JSONL/log → SQLite 変換
└── backfill/                      # PR URL 一括補完（gh pr list 並列実行）
grafana/                       # Grafana ダッシュボード定義
├── dashboards/claudedog.json      # ダッシュボード JSON（11パネル）
└── provisioning/                  # datasource・dashboard provisioning
e2e/                           # E2E テスト
├── testdata/                      # テストフィクスチャ
├── gen_testdb_test.go             # テスト用 SQLite DB 生成
└── screenshot.sh                  # Grafana スクリーンショット取得
```

## CLI コマンド

```
claudedog update <session_id> <url>...          # PR URL を追加
claudedog update --mark-checked <session_id>... # backfill_checked をセット
claudedog update --by-branch <repo> <branch> <url>  # ブランチ全セッションに URL 追加
claudedog backfill [--recheck]                  # PR URL の一括補完
claudedog sync-db                               # JSONL/log → SQLite 変換
```

## セットアップ

```fish
# symlink 作成
ln -sf (ghq root)/github.com/ishii1648/claudedog ~/.claude/claudedog

# ビルド＆配置
go build -o ~/.local/bin/claudedog ./cmd/claudedog/

# DB 生成 & ダッシュボード確認
claudedog sync-db
# Grafana で ~/.claude/claudedog.db を SQLite datasource として設定
```

## E2E テスト（Grafana スクリーンショット検証）

```fish
make grafana-up          # Docker で Grafana + Image Renderer 起動 → http://localhost:13000
make grafana-screenshot  # 全11パネルの PNG を .outputs/grafana-screenshots/ に取得
make grafana-down        # コンテナ停止
```

## 経緯

元は [dotfiles](https://github.com/ishii1648/dotfiles) リポジトリの `configs/claude/scripts/` に分散していた計測スクリプト群。ADR-052 でトップレベルディレクトリに隔離後、別リポジトリに完全分離。その後 Python 実装を Go にリライトし、ダッシュボードを Streamlit から Grafana（SQLite datasource）に移行（ADR-015）。
