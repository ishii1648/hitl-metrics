---
name: impl
description: Use when the user says "impl", "実装", "実装セッション起動",
  "TODO実装して", or wants to launch implementation sessions for TODO tasks.
  Parses TODO.md and launches worktree + tmux session + Codex sessions.
version: 0.1.0
---

# impl

TODO.md の `実装タスク` セクションを検出し、タスクごとに worktree + tmux session + Codex を起動して並列実装をディスパッチする。

## 前提条件

- tmux セッション内で実行されていること
- main ブランチ上で実行されていること（設計セッション）

## ステップ

### Step 1: 前提チェック

- 現在のブランチが main であることを確認する（`git branch --show-current`）
- main でない場合は「impl は設計セッション（main ブランチ）から実行してください」と報告して終了

### Step 2: TODO.md パース

`$PWD/TODO.md` を Read して `実装タスク` セクションからタスクを抽出する。`検討中` セクションのタスクは仕様未確定のため対象外。作業中タスクの識別はブランチ・Draft PR で行うため TODO.md 上の状態セクションは持たない。

**タスクの識別ルール:**

- `- ` で始まる行がタスクの先頭行（タイトル）
- `  - ` で始まる後続行はタスクの詳細（同一タスクに属する）
- 空行または次の `## ` 見出しでセクション終了

**ブランチ名の導出:**

- タイトルからブランチ名を自動生成（日本語をキーワード化して kebab-case）
- ブランチプレフィックス: タイトルに「修正」「fix」を含む → `fix/`、それ以外 → `feat/`

### Step 3: ブランチ存在チェック

各タスクについて以下を確認する:

- `git branch -a` で対応ブランチが存在するか
- `git worktree list` で対応 worktree が存在するか
- 既に存在するタスクは「スキップ」としてマークする

### Step 4: 対象タスク一覧の提示

検出したタスクを一覧表示する:

```
impl 対象タスク:

  1. session-index.jsonl の書き戻しを atomic 化
     ブランチ: feat/atomic-session-index-write

  2. Bash コマンドのコンテキスト消費監視
     ブランチ: feat/bash-context-monitoring

  スキップ（既存ブランチ）:
  - feat/some-existing-task（worktree 存在）
```

args に `--dry-run` が含まれる場合はここで終了する。

args に `--dry-run` が含まれない場合は AskUserQuestion で「これらのタスクを impl しますか？ 番号で選択するか、all で全件実行」と確認する。

### Step 5: worktree + tmux session 作成

選択されたタスクごとに以下を順次実行する:

#### 5-1: worktree 作成

```bash
git fetch origin
git worktree add "<main_worktree_path>@<branch_dir_name>" -b "<branch_name>" origin/HEAD
```

- `<main_worktree_path>`: `git worktree list --porcelain | head -1` から取得
- `<branch_dir_name>`: ブランチ名の `/` を `-` に置換（例: `feat/adr-017` → `feat-adr-017`）
- `<branch_name>`: Step 2 で導出したブランチ名

#### 5-2: settings.local.json コピー

```bash
cp .Codex/settings.local.json "<worktree_path>/.Codex/settings.local.json"
```

（`.Codex/settings.local.json` が存在する場合のみ）

#### 5-3: tmux window 作成

現在の tmux session にウィンドウを追加する。

```bash
tmux new-window -n "<window_name>" -c "<worktree_path>"
```

- `<window_name>`: ブランチ名の `/` を `-` に置換（例: `feat/adr-017` → `feat-adr-017`）

### Step 6: Codex 起動と初期プロンプト送信

各 tmux window に対して以下を実行する:

#### 6-1: Codex 起動

```bash
tmux send-keys -t "<window_name>" "Codex" Enter
```

#### 6-2: 初期プロンプト送信（Codex 起動待ち後）

Codex の起動を 5 秒待ってから初期プロンプトを送信する。

TODO.md から抽出したタスクタイトル・詳細行を含む初期プロンプトを構築する。

```bash
sleep 5
tmux send-keys -t "<window_name>" "<初期プロンプト>" Enter
```

**初期プロンプトのテンプレート:**

```
TODO.md の以下のタスクを実装してください。

タスク: <タスクタイトル>

詳細:
- 詳細1
- 詳細2

全ての要件を満たすまで実装→テスト→検証を繰り返してください。
完了したら Draft PR を作成してください（TODO.md からの完了タスク削除は次回 main で SessionStart hook が `todo-cleanup` 経由で自動的に行います）。
```

タスクタイトルは TODO.md の先頭行、詳細は `  - ` で始まるサブ行から抽出する。
受け入れ条件（`- [ ]`）がある場合はそれも含める。

### Step 7: 完了報告

起動したウィンドウの一覧を報告する:

```
impl 完了:

  1. feat-atomic-session-index-write → 初期プロンプト送信済み
  2. feat-bash-context-monitoring → 初期プロンプト送信済み

tmux window 一覧: tmux list-windows
切替: tmux select-window -t <window_name>
```

## 制約

- 操作対象は常に `$PWD` 配下
- impl は main ブランチからのみ実行可能
- `gw_add` は tmux switch-client を含むため直接呼び出さない。worktree 作成・tmux window 作成・settings コピーを個別に実行する
- 各タスクの worktree 作成は順次実行する（git worktree add は並列実行不可）
- `tmux send-keys` での初期プロンプト送信は Codex の起動待ち（5秒）が必要
