# ADR-017: 設計/実装セッション分離の自動ディスパッチ

## ステータス

採用済み

## 関連 ADR

- 関連: ADR-013（開発プロセスの分離を前提）

## コンテキスト

claudedog の開発では、設計セッション（main ブランチで ADR を作成）と実装セッション（worktree で ADR を実装）を分離する運用を導入した（CLAUDE.md セッションモード規約）。

現状、実装セッションの起動は手動で行っている：

1. `gw_add feat/adr-NNN` で worktree + tmux session 作成
2. Claude Code を起動して ADR 内容を手動で伝達
3. 実装完了後に PR 作成

この手動プロセスを自動化し、設計セッション完了後に one-shot コマンドで複数の ADR 実装を並列ディスパッチできるようにしたい。

また、ADR と TODO.md の役割分担が曖昧になっている。ADR は設計判断の記録、TODO.md は実装タスクの管理という役割を明確化し、dispatch の入力ソースを TODO.md に一本化する。

## 設計案

### 案A: one-shot dispatch skill + TODO.md 駆動（採用）

人間が設計セッション完了後に明示的に実行する one-shot dispatch 方式。dispatch の入力ソースは TODO.md に一本化する。

**ADR と TODO.md の役割分担:**

| ドキュメント | 役割 | dispatch との関係 |
|---|---|---|
| ADR (`docs/adr/`) | 設計判断の「なぜ」を記録する履歴ログ。設計判断を伴う場合にのみ作成 | ADR 付きタスクは ADR の受け入れ条件で完了判定 |
| TODO.md | 全実装タスクの「何を」管理するフロー | dispatch の入力ソース（ADR 有無を問わない） |

**TODO.md のタスク種別:**

- **ADR 付きタスク**: 設計判断を伴うもの。ADR リンクと受け入れ条件への参照を記載
- **ADR なしタスク**: バグ修正・設定変更・軽微な改善など。タスク説明のみで dispatch 可能

**ADR ステータスと TODO.md の関係:**

- ADR `採用済み` / `Spike中` → TODO.md に追記する（実装 or 検証の対象）
- ADR `Draft` → TODO.md には載せない（設計未完了）

**フロー:**

1. 設計セッション（main）でタスクを TODO.md の「未着手」に追記する（必要に応じて ADR も作成）
2. 人間が設計セッション内で `/dispatch` skill を実行
3. skill が TODO.md の「未着手」セクションから dispatch 対象タスクを検出
4. タスクごとに `gw_add -c` で worktree + tmux session + Claude Code を起動
5. Claude Code に TODO.md の受け入れ条件を含む初期プロンプトを `tmux send-keys` で渡す（ADR 有無による分岐なし）
6. 各 Claude session が実装 → テスト → Draft PR 作成
7. 人間が PR レビュー → マージ
8. 設計セッション（main）で `git pull` → TODO.md から完了タスク削除・CHANGELOG 更新

**dispatch の検出ロジック:**

- TODO.md の「未着手」セクションから全タスクを抽出
- タスク名からブランチ名を導出（ADR 付き → `feat/adr-NNN`、ADR なし → `feat/` or `fix/` + kebab-case）
- `git branch -a` で対応ブランチが存在しないことを確認
- 既に worktree/ブランチが存在するタスクはスキップ

### 案B: watcher/polling 方式（却下）

ファイル監視や定期ポーリングで Ready 状態の TODO/ADR を自動検出する方式。

却下理由:
- 「意図しないタイミングで実装が走る」リスクがある
- daemon/cron の管理コストが発生する
- 設計セッション完了は人間が判断するもので、自動検出に馴染まない

### 案C: plan mode ゲート付きディスパッチ（却下）

dispatch 後に Claude Code を plan mode で起動し、実装計画を人間が承認してから実装を開始する方式。

却下理由:
- ADR に受け入れ条件・変更ファイル一覧・設計案が既にある場合、ADR 自体がプラン
- plan mode での計画承認は二重チェックになり、並列実装のメリットを減殺する
- ADR の設計が粗い場合はそもそも `採用済み` にすべきでない（設計セッションの責務）

### 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `.claude/skills/dispatch/` | dispatch skill 定義（新規） |
| `.claude/skills/adr-ship/` | 削除（dispatch の初期プロンプトに統合） |
| `.claude/skills/create-adr/` | 受け入れ条件の記載先を ADR → TODO.md に変更 |
| `CLAUDE.md` | セッションモード規約に dispatch フロー追記、adr-ship 参照除去 |
| `.claude/skills/adr-reference/skill.md` | 受け入れ条件の SSOT を TODO.md に変更、adr-ship 参照除去 |
