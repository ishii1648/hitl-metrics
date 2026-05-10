---
title: data-flow
weight: 30
---

セッション開始から PR 単位の集計値が出るまでの**データの流れ**を追います。各 agent の transcript フォーマットの違いをどこで吸収しているか、PR への紐付け（pinning）がいつ確定するかを把握できます。

## 一回のセッションのライフサイクル

```mermaid
sequenceDiagram
    autonumber
    participant U as ユーザー
    participant A as Coding Agent<br/>(Claude / Codex)
    participant H as hook<br/>(agent-telemetry hook ...)
    participant J as session-index.jsonl<br/>+ transcript
    participant CLI as agent-telemetry CLI
    participant DB as SQLite

    U->>A: セッション開始
    A->>H: SessionStart 発火
    H->>J: メタデータ追記<br/>(session_id, cwd, repo, branch, user_id)

    loop 応答ターン
        A->>A: tool_use / message 生成<br/>token usage を transcript に記録
        opt Codex のみ
            A->>H: PostToolUse
            H->>J: tool_response から PR URL を抽出 → pr_urls 追記
        end
        A->>H: Stop（応答完了）
        H->>H: branch から `gh pr list --head` で PR を解決
        H->>J: pr_pinned: true で確定
        H->>CLI: backfill → sync-db (blocking)
        CLI->>J: pr_urls / is_merged / review_comments 補完
        CLI->>DB: sessions / transcript_stats を upsert
    end

    A->>H: SessionEnd（Claude のみ）
    H->>J: ended_at / end_reason 確定
```

ポイント:

- **PR pinning は `Stop` hook で確定する** — 応答完了時点の branch を `gh pr list --head` で問い合わせて PR を確定させ、`pr_pinned: true` を立てる。pinned 後は `PostToolUse` や `update` での URL 追記を no-op 化して**誤接続を防ぐ**
- **`backfill` → `sync-db` はブロッキング** — 応答が返るまでに DB が最新化される。UX 上の遅延と引き換えに「table を見れば直前のセッションが映っている」整合性を取っている

## agent ごとの transcript パース

Claude と Codex で transcript フォーマットは異なります。**`internal/transcript/`** がフォーマット差異を吸収して `transcript_stats` の共通カラムに落とし込みます。

```mermaid
flowchart TB
    subgraph claude["Claude Code transcript"]
        CM["assistant message<br/>usage.input_tokens<br/>usage.output_tokens<br/>usage.cache_creation_input_tokens<br/>usage.cache_read_input_tokens"]
    end

    subgraph codex["Codex CLI rollout"]
        CXE["event_msg.payload<br/>type == 'token_count'<br/>(累積値の最終 1 件を採用)"]
    end

    P["internal/transcript/<br/>(parser)"]

    subgraph stats["transcript_stats（共通スキーマ）"]
        T1["input_tokens / output_tokens"]
        T2["cache_write_tokens / cache_read_tokens"]
        T3["reasoning_tokens<br/>(Claude=0, Codex のみ非ゼロ)"]
        T4["tool_use_total / mid_session_msgs"]
        T5["ask_user_question<br/>(Codex=0, Claude のみ非ゼロ)"]
    end

    CM --> P
    CXE --> P
    P --> T1
    P --> T2
    P --> T3
    P --> T4
    P --> T5
```

差異の扱い:

| 軸 | Claude | Codex |
|---|---|---|
| token 集計の単位 | message ごとの `usage` を**合算** | rollout の `token_count` イベントの**最終累積値**を採用 |
| `reasoning_tokens` | 常に 0 | 非ゼロを取りうる |
| `ask_user_question` | 非ゼロを取りうる | 常に 0（Codex に該当 tool 概念がないため） |
| transcript ファイル | `~/.claude/projects/**/<session-id>.jsonl` | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl[.zst]` |

zstd 圧縮された Codex rollout は decoder を通して読みます（`klauspost/compress/zstd`）。

## sessions → pr_metrics の集約

`pr_metrics` VIEW で PR 単位に集約します。フィルタを通過したセッションだけが指標に乗ります。

```mermaid
flowchart LR
    S[("sessions")]
    TS[("transcript_stats")]

    F1{"pr_url != ''"}
    F2{"is_subagent = 0"}
    F3{"is_merged = 1"}
    F4{"is_ghost = 0"}
    F5{"repo NOT IN<br/>(dotfiles)"}

    G["GROUP BY<br/>(pr_url, coding_agent, user_id)"]

    PM[("pr_metrics<br/>VIEW")]

    S --> F1 --> F2 --> F3
    TS --> F4
    F3 --> F5
    F4 --> F5
    F5 --> G
    G --> PM
```

なぜこのフィルタか:

- `pr_url != ''` — PR 未作成セッションを除外（PR 単位の効率を見るため）
- `is_subagent = 0` — サブエージェントセッションは親と二重計上になるので除外
- `is_merged = 1` — 未マージ・放棄 PR は最終成果物ではないため除外
- `is_ghost = 0` — ユーザー発話相当が 0 件のセッション（環境調査だけで終わった等）を除外
- dotfiles 除外 — agent-telemetry の運用上の自明なノイズ

集約軸が `(pr_url, coding_agent, user_id)` の 3 軸なのは、同一 PR が複数 agent / 複数ユーザに触られたケース（pair coding 等）を意味的に分離するためです。実運用上ほぼ発生しませんが、起きたときに合算してしまうと指標が歪むので分離しています。
