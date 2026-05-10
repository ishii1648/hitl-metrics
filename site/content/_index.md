---
title: agent-telemetry
type: docs
---

# agent-telemetry

Claude Code および Codex CLI を使った開発で、**PR 単位のトークン消費効率**を追跡・可視化する計測ツールの仕組み解説 site です。

このサイトはツールの **how it works**（hook 構成・データフロー・dashboard の読み方）に絞った解説 docs を提供します。CLI 仕様 / 設計判断 / セットアップ手順といった reference 系ドキュメントは引き続き [GitHub repo の `docs/`](https://github.com/ishii1648/agent-telemetry/tree/main/docs) を正とします。

## どこから読むか

- 全体像から知りたい → [architecture]({{< relref "/explain/architecture" >}})
- データがどう流れているか追いたい → [data-flow]({{< relref "/explain/data-flow" >}})
- どの hook が何を集めているか → [hooks]({{< relref "/explain/hooks" >}})
- dashboard の見方を知りたい → [dashboard]({{< relref "/explain/dashboard" >}})

## reference 系ドキュメント（GitHub）

- [docs/spec.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md) — 外部契約（CLI・hook 仕様・データモデル）
- [docs/metrics.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/metrics.md) — 計測フレームワーク
- [docs/design.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/design.md) — 実装方針と設計判断
- [docs/setup.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/setup.md) — セットアップ手順
- [docs/usage.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/usage.md) — 日常運用
