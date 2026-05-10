---
title: hooks
weight: 20
---

agent-telemetry が登録する hook の一覧と、それぞれが**何のイベントを契機に・何のデータを集めるか**をまとめます。hook は agent プロセスから同期的に呼ばれるため、**永続化以外の重い処理を入れない**設計です（PR 集約は `sync-db` 側で行う）。

## hook がカバーする範囲

```mermaid
flowchart TB
    subgraph life["セッションのライフサイクル"]
        S0["セッション開始"]
        S1["応答ターン 1<br/>(tool_use)"]
        S2["応答ターン 2<br/>(message)"]
        S3["応答ターン N"]
        S4["セッション終了"]
    end

    subgraph claude["Claude Code hooks"]
        CSS["SessionStart"]
        CST["Stop<br/>(各応答完了)"]
        CSE["SessionEnd"]
    end

    subgraph codex["Codex CLI hooks"]
        XSS["SessionStart<br/>(startup / resume)"]
        XPT["PostToolUse"]
        XST["Stop<br/>(各応答完了)"]
    end

    S0 --> CSS
    S0 --> XSS
    S1 --> CST
    S1 --> XPT
    S1 --> XST
    S2 --> CST
    S2 --> XST
    S3 --> CST
    S3 --> XST
    S4 --> CSE
    S4 -. Codex は SessionEnd なし<br/>最後の Stop が SessionEnd 相当 .-> XST
```

Codex は `SessionEnd` を持たないため、`Stop` hook で `ended_at` を毎回上書きします。最後に発火した `Stop` がそのまま「終了時刻」になります。

## hook と用途の対応表

`agent-telemetry hook <event> --agent <claude|codex>` のサブコマンド形式で呼ばれます。`agent-telemetry` バイナリが PATH 上に必要です。

### Claude Code

| hook | サブコマンド | 用途 |
|---|---|---|
| `SessionStart` | `hook session-start --agent claude` | セッション開始メタデータ（`session_id` / `cwd` / `repo` / `branch` / `user_id`）を `~/.claude/session-index.jsonl` に追記 |
| `SessionEnd` | `hook session-end --agent claude` | `ended_at` / `end_reason` を確定し `sync-db` を実行 |
| `Stop` | `hook stop --agent claude` | branch から PR を解決して `pr_pinned: true` で確定 → `backfill` → `sync-db`（ブロッキング） |

### Codex CLI

| hook | サブコマンド | 用途 |
|---|---|---|
| `SessionStart` (`startup` / `resume`) | `hook session-start --agent codex` | セッション開始メタデータを `~/.codex/session-index.jsonl` に追記 |
| `PostToolUse` | `hook post-tool-use --agent codex` | `tool_response` 文字列から PR URL を抽出して `pr_urls` に追記（`pr_pinned: true` のセッションでは no-op） |
| `Stop` | `hook stop --agent codex` | branch から PR を解決して `pr_pinned: true` で確定し `ended_at` を更新 → `backfill` → `sync-db`（ブロッキング） |

## `Stop` hook の処理時間

`Stop` hook は応答完了ごとに `backfill` → `sync-db` をブロッキングで走らせます。応答が長引かないよう **3 つの抑制策**が入っています。

```mermaid
flowchart LR
    A["Stop hook 発火"]
    B{"cursor で<br/>未処理セッションだけ<br/>抽出"}
    C{"時間条件で<br/>スキップ判定"}
    D{"goroutine 並列で<br/>gh CLI 呼び出し"}
    E{"8 秒タイムアウト"}
    F["JSONL 書き戻し"]
    G["sync-db で SQLite 更新"]

    A --> B --> C --> D --> E --> F --> G
```

| 抑制策 | 効果 |
|---|---|
| cursor 方式 | 既に `backfill_checked: true` のセッションは再 API 呼び出ししない |
| 時間条件スキップ | 直近 N 分以内に走った場合はスキップ |
| goroutine 並列 | 複数セッションの `gh pr view` 等を並列発行 |
| 8 秒タイムアウト | 全体で 8 秒以上かかったら強制打ち切り（hook 完了を優先） |

それでも長引く環境では `Stop` を非同期化するアイデアもあるが、**「応答が返る頃には DB が最新」**という整合性を優先して同期実行に振っています。

## PR URL 解決の優先順位

PR URL は複数の経路から到達するため、衝突を避けるために優先順位が決まっています。

```mermaid
flowchart TB
    A["Stop hook<br/>(gh pr list --head branch)"]
    B["PostToolUse hook<br/>(tool_response から抽出)"]
    C["agent-telemetry update CLI<br/>(手動指定)"]
    D["agent-telemetry backfill<br/>(後追い補完)"]

    P{"pr_pinned: true ?"}
    JSONL[("session-index.jsonl<br/>pr_urls[]")]

    A -- "pinned 確定" --> JSONL
    B --> P
    C --> P
    D --> P

    P -- "yes (no-op)" --> X["何もしない"]
    P -- "no" --> JSONL
```

`Stop` hook が `pr_pinned: true` を立てた後は、他経路からの URL 追記は**すべて no-op** になります。これは「branch とは無関係に PR URL が混入する」事故を防ぐためで、たとえば PR コメントに別 PR のリンクを貼った瞬間に `PostToolUse` が誤検出して紐付けが壊れる、というケースを排除します。

## hook 登録のしくみ

`agent-telemetry setup` は登録例を**表示するだけ**で、自動書き込みはしません。ユーザーが dotfiles または手動で登録する前提です。

- Claude Code: `~/.claude/settings.json` の `hooks` セクションに追記
- Codex CLI: `~/.codex/config.toml` に `[features] codex_hooks = true` を立てたうえで `[[hooks.<Event>]]` を追加、または `~/.codex/hooks.json` を配置

過去 `agent-telemetry install` 系統で自動登録された hook がある場合は、`agent-telemetry doctor` の legacy hook warning を頼りに手動で削除してください（自動削除は提供しません）。

具体的なセットアップ手順は [setup/local]({{< relref "/setup/local" >}}) を参照してください。
