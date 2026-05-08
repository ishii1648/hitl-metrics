---
name: impl
description: Use when the user says "impl", "実装", "実装セッション起動",
  "issue実装して", or wants to launch implementation sessions for open issues.
  Parses issues/ and launches worktree + tmux session + Codex sessions.
version: 0.2.0
---

# impl

`issues/` 直下の open issue（`issues/<NNNN>-<cat>-<slug>.md`）を検出し、issue ごとに worktree + tmux session + Codex を起動して並列実装をディスパッチする。`issues/closed/` と `issues/pending/` は対象外。

## 前提条件

- tmux セッション内で実行されていること
- main ブランチ上で実行されていること（設計セッション）

## ステップ

### Step 1: 前提チェック

- 現在のブランチが main であることを確認する（`git branch --show-current`）
- main でない場合は「impl は設計セッション（main ブランチ）から実行してください」と報告して終了

### Step 2: open issue の列挙

`$PWD/issues/*.md` を読み（`closed/` と `pending/` のサブディレクトリは除外）、各ファイルから以下を抽出する:

- ファイル名: `<NNNN>-<cat>-<slug>.md`（4 桁の連番 + カテゴリ + スラッグ）
- タイトル: 1 行目の `# <タイトル>`
- 受け入れ条件: `## 受け入れ条件` セクションの `- [ ]` / `- [x]` 行
- 詳細: `## 対応方針` セクションの本文

`SEQUENCE` ファイルは無視する。

**ブランチ名の導出:**

- ファイル名のスラッグからブランチ名を生成する（`<NNNN>-<cat>-<slug>` の `<cat>` を prefix、`<slug>` を suffix にする。例: `0003-feat-grafana-agent-comparison-panel.md` → `feat/grafana-agent-comparison-panel`）
- `<cat>` が `bug` の場合は `fix/` を prefix にする
- `<cat>` が `chore` / `doc` の場合はそれぞれ `chore/` / `docs/` を prefix にする

### Step 3: ブランチ存在チェック

各 issue について以下を確認する:

- `git branch -a` で対応ブランチが存在するか
- `git worktree list` で対応 worktree が存在するか
- 既に存在する issue は「スキップ」としてマークする

### Step 4: 対象 issue 一覧の提示

検出した issue を一覧表示する:

```
impl 対象 issue:

  0003. Grafana ダッシュボードに agent 別比較 stat パネルを追加
        ブランチ: feat/grafana-agent-comparison-panel

  0004. GoReleaser tag push 動作確認 + リリース
        ブランチ: chore/goreleaser-release-verification

  スキップ（既存ブランチ）:
  - feat/some-existing-task（worktree 存在）
```

args に `--dry-run` が含まれる場合はここで終了する。

args に `--dry-run` が含まれない場合は AskUserQuestion で「これらの issue を impl しますか？ 番号で選択するか、all で全件実行」と確認する。

### Step 5: worktree + tmux session 作成

選択された issue ごとに以下を順次実行する:

#### 5-1: worktree 作成

```bash
git fetch origin
git worktree add "<main_worktree_path>@<branch_dir_name>" -b "<branch_name>" origin/HEAD
```

- `<main_worktree_path>`: `git worktree list --porcelain | head -1` から取得
- `<branch_dir_name>`: ブランチ名の `/` を `-` に置換（例: `feat/grafana-agent-comparison-panel` → `feat-grafana-agent-comparison-panel`）
- `<branch_name>`: Step 2 で導出したブランチ名

#### 5-2: settings.local.json コピー

```bash
cp .codex/settings.local.json "<worktree_path>/.codex/settings.local.json"
```

（`.codex/settings.local.json` が存在する場合のみ）

#### 5-3: tmux window 作成

現在の tmux session にウィンドウを追加する。

```bash
tmux new-window -n "<window_name>" -c "<worktree_path>"
```

- `<window_name>`: ブランチ名の `/` を `-` に置換

### Step 6: Codex 起動と初期プロンプト送信

各 tmux window に対して以下を実行する:

#### 6-1: Codex 起動

```bash
tmux send-keys -t "<window_name>" "codex" Enter
```

#### 6-2: 初期プロンプト送信（Codex 起動待ち後）

Codex の起動を 5 秒待ってから初期プロンプトを送信する。

issue ファイルから抽出したタイトル・対応方針・受け入れ条件を含む初期プロンプトを構築する。

```bash
sleep 5
tmux send-keys -t "<window_name>" "<初期プロンプト>" Enter
```

**初期プロンプトのテンプレート:**

```
issues/<NNNN>-<cat>-<slug>.md の issue を実装してください。

タイトル: <タイトル>

対応方針:
<対応方針セクションの本文>

受け入れ条件:
- [ ] 条件1
- [ ] 条件2

全ての受け入れ条件を満たすまで実装→テスト→検証を繰り返してください。
完了したら最終コミットで `git mv issues/<id>-... issues/closed/<id>-...` し、末尾に `Completed: YYYY-MM-DD` と `## 解決方法` を追記したうえで Draft PR を作成してください。
```

### Step 7: 完了報告

起動したウィンドウの一覧を報告する:

```
impl 完了:

  1. feat-grafana-agent-comparison-panel → 初期プロンプト送信済み
  2. chore-goreleaser-release-verification → 初期プロンプト送信済み

tmux window 一覧: tmux list-windows
切替: tmux select-window -t <window_name>
```

## 制約

- 操作対象は常に `$PWD` 配下
- impl は main ブランチからのみ実行可能
- `gw_add` は tmux switch-client を含むため直接呼び出さない。worktree 作成・tmux window 作成・settings コピーを個別に実行する
- 各 issue の worktree 作成は順次実行する（git worktree add は並列実行不可）
- `tmux send-keys` での初期プロンプト送信は Codex の起動待ち（5秒）が必要
- `issues/pending/` 配下の issue は仕様未確定のため対象外
