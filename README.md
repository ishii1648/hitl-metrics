# claudedog

Claude Code の人の介入率を追跡・可視化する計測ツール（hook・バッチ・ダッシュボード）。

## 構成

```
hooks/                     # Claude Code hook スクリプト（データ収集層）
├── session-index.sh           # セッション開始時に JSONL へ記録
├── permission-log.sh          # PermissionRequest hook で permission UI をログ
└── pretooluse-track.sh        # PreToolUse hook でツール名を記録
batch/                     # cron/バッチ処理（データ補完層）
├── session-index-update.py    # セッション終了検知・メトリクス更新
└── session-index-backfill-batch.py  # PR URL バックフィル（並列）
dashboard/                 # Web UI（可視化層）
└── server.py                  # Streamlit ダッシュボード
claudedog                  # CLI エントリポイント
```

## セットアップ

### 1. symlink 作成

```fish
ln -sf (ghq root)/github.com/ishii1648/claudedog ~/.claude/claudedog
ln -sf ~/.claude/claudedog/claudedog ~/.local/bin/claudedog
```

### 2. 動作確認

```fish
# hook 動作確認（Claude Code 起動後）
tail -1 ~/.claude/session-index.jsonl

# ダッシュボード起動
claudedog start
# → http://localhost:18765
```

## 経緯

元は [dotfiles](https://github.com/ishii1648/dotfiles) リポジトリの `configs/claude/scripts/` に分散していた計測スクリプト群。ADR-052 でトップレベルディレクトリに隔離後、結合度が十分に低下したため別リポジトリに完全分離。

## 開発プロセス

- ADR は作成しない。`TODO.md` に「やりたいこと + 完了条件」を箇条書き
- 完了したら `CHANGELOG.md` に記録して TODO から削除
- 意思決定の追跡は git log + CHANGELOG で十分
- 過去の ADR は `docs/adr/` に参照用として保持
