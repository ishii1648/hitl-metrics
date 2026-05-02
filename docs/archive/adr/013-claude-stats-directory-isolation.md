# ADR-013: hitl-metrics をトップレベルディレクトリに隔離し開発プロセスを分離する

## ステータス
部分廃止 — 案A（ディレクトリ隔離）は採用・実施済み。案B（別リポジトリ分離）は当初却下したが、隔離実施後に結合度が十分低下したため覆し、別リポジトリとして完全分離した（2026-03-09）。

## 関連 ADR
- 関連: ADR-001（session-index の基盤）
- 関連: ADR-003（permission UI 計測の基盤）
- 関連: ADR-007（人の介入指標拡張）
- 関連: ADR-008（時系列トレンドグラフ）
- 関連: ADR-009（permission UI 内訳監視）
- 関連: ADR-011（サブエージェントセッション除外）
- 関連: ADR-012（ツール別テーブル）

## コンテキスト

hitl-metrics（旧 claude-stats）は hook/cron 経由で収集した拡張データをもとに Claude Code の人の介入率を追跡・可視化する計測ツールだが、次の問題がある。

1. **ADR 駆動開発との不一致**: UI 調整やメトリクス追加など試行錯誤的な開発が多く、毎回 ADR を作成するのは冗長
2. **ADR の占有**: docs/adr/ 全体 51 件中 7 件（約 14%）が hitl-metrics 関連で、今後も増加傾向
3. **実装の散在**: `configs/claude/scripts/` に 8 ファイルが他の Claude Code スクリプト（approve-safe-commands.py, redirect-to-tools.py 等）と混在しており、ディレクトリレベルでの凝集性が乏しい

別リポジトリへの分離も検討したが、hitl-metrics は hook 登録（`settings.json`）やシンボリックリンク管理など dotfiles と密結合しているため、リポジトリ内でのディレクトリ隔離が妥当と判断した。

また、ディレクトリ隔離に伴い開発プロセスも分離する。dotfiles 本体は引き続き ADR 駆動で開発するが、hitl-metrics は TODO.md + CHANGELOG.md ベースの軽量プロセスに移行する。ADR の「受け入れ条件による完了判定」は TODO.md の完了条件として軽量に継承し、意思決定の履歴は git log + CHANGELOG で追跡する。

## 設計案

### 案A: トップレベル `hitl-metrics/` ディレクトリに集約（採用）

```
hitl-metrics/
├── hooks/                     # Claude Code hook スクリプト（データ収集層）
│   ├── session-index.sh           <- configs/claude/scripts/session-index.sh
│   ├── permission-log.sh          <- configs/claude/scripts/permission-log.sh
│   └── pretooluse-track.sh        <- configs/claude/scripts/pretooluse-track.sh
├── batch/                     # cron/バッチ処理（データ補完層）
│   ├── session-index-update.py    <- configs/claude/scripts/session-index-update.py
│   └── session-index-backfill-batch.py <- configs/claude/scripts/session-index-backfill-batch.py
├── dashboard/                 # Web UI（可視化層）
│   └── server.py                  <- configs/claude/scripts/permission-ui-server.py
├── hitl-metrics                  <- configs/claude/scripts/claude-stats（CLI エントリポイント）
├── TODO.md                    # やること + 完了条件（事前）
├── CHANGELOG.md               # 変更履歴（事後）
└── README.md
```

- `configs/claude/settings.json` の hook パスを `hitl-metrics/hooks/` を指すように変更
- `configs/claude/scripts/` には hitl-metrics 以外のスクリプトのみ残す

**開発プロセスの変更:**
- hitl-metrics の変更は ADR を作成しない。`TODO.md` に「やりたいこと + 完了条件」を箇条書きし、完了後に `CHANGELOG.md` へ記録する
- `docs/development.md` に「hitl-metrics ディレクトリ内の変更は ADR 不要、TODO.md + CHANGELOG.md で管理」ルールを追記
- 既存の hitl-metrics 関連 ADR（001, 003, 007-009, 011, 012）はそのまま残す（過去の意思決定記録として有効）

### 案B: 別リポジトリに完全分離（当初却下 → 後に採用）

hook 登録・シンボリックリンク管理が dotfiles 側にあるため、当初は install.sh で結合を再構築する手間が増えるだけで実質的なメリットが薄いと判断し却下した。

しかし案A（ディレクトリ隔離）実施後、以下の点が明らかになり別リポジトリ化が現実的になった:

1. hook パスは `~/.claude/hitl-metrics/hooks/*` 経由で動作するため、symlink 先が dotfiles 内か別リポジトリかに依存しない
2. `setup-manifest.yml` の symlink 2 行を変更するだけで切り替え可能
3. hitl-metrics 関連 ADR 14 件が dotfiles のコンテキストを不要に増大させていた
4. 開発プロセスが既に TODO.md + CHANGELOG.md に分離済みで、ADR 駆動と完全に独立していた

### 変更が必要なファイル
| ファイル | リポジトリ | 変更内容 |
|---|---|---|
| `hitl-metrics/` | dotfiles | 新規ディレクトリ作成、8 ファイル移動 |
| `configs/claude/scripts/` | dotfiles | hitl-metrics 関連 8 ファイル削除 |
| `configs/claude/settings.json` | dotfiles | hook パスを `hitl-metrics/hooks/` に変更 |
| `docs/development.md` | dotfiles | hitl-metrics の開発プロセス（TODO.md + CHANGELOG.md）ルール追記 |
| `hitl-metrics/TODO.md` | dotfiles | 新規作成 |
| `hitl-metrics/CHANGELOG.md` | dotfiles | 新規作成 |
| `docs/reference.md` | dotfiles | hitl-metrics コンポーネントの記載更新 |

## 受け入れ条件
- [x] hitl-metrics が独立ディレクトリに集約されている
- [x] 開発プロセスが TODO.md + CHANGELOG.md に移行されている
