---
title: agent-telemetry
toc: false
cascade:
  type: docs
---

Claude Code および Codex CLI を使った開発で、**PR 単位のトークン消費効率**を追跡・可視化する計測ツールの仕組み解説 site です。

このサイトはツールの **how it works**（hook 構成・データフロー・dashboard の読み方）に絞った解説 docs を提供します。CLI 仕様 / 設計判断 / セットアップ手順といった reference 系ドキュメントは引き続き [GitHub repo の `docs/`](https://github.com/ishii1648/agent-telemetry/tree/main/docs) を正とします。

{{< cards >}}
  {{< card link="/explain/metrics" title="metrics" subtitle="何を観察しているか・なぜそれを選んだか" >}}
  {{< card link="/explain/architecture" title="architecture" subtitle="全体像（hook → JSONL → SQLite → Grafana）" >}}
  {{< card link="/explain/hooks" title="hooks" subtitle="どの hook がどのデータを何に使うか" >}}
  {{< card link="/explain/data-flow" title="data-flow" subtitle="Claude / Codex の transcript パースから集約まで" >}}
  {{< card link="/explain/dashboard" title="dashboard" subtitle="Grafana dashboard の panel ごとの読み方" >}}
{{< /cards >}}

## reference 系ドキュメント（GitHub）

- [docs/spec.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md) — 外部契約（CLI・hook 仕様・データモデル）
- [docs/metrics.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/metrics.md) — 計測フレームワーク
- [docs/design.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/design.md) — 実装方針と設計判断
- [docs/setup.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/setup.md) — セットアップ手順
- [docs/usage.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/usage.md) — 日常運用
