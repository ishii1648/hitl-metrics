---
title: agent-telemetry
toc: false
cascade:
  type: docs
---

Claude Code および Codex CLI を使った開発で、**PR 単位のトークン消費効率**を追跡・可視化する計測ツールの解説 site です。

このサイトは **how it works**（観察軸・hook 構成・データフロー）と **how to use**（セットアップ・運用）を提供します。CLI 仕様・データモデル・設計判断といった reference 系ドキュメントは引き続き [GitHub repo の `docs/`](https://github.com/ishii1648/agent-telemetry/tree/main/docs) を正とします。

## 仕組み解説

{{< cards >}}
  {{< card link="/explain/metrics" title="metrics" subtitle="何を観察しているか・なぜそれを選んだか" >}}
  {{< card link="/explain/architecture" title="architecture" subtitle="全体像（hook → JSONL → SQLite → Grafana）" >}}
  {{< card link="/explain/hooks" title="hooks" subtitle="どの hook がどのデータを何に使うか" >}}
  {{< card link="/explain/data-flow" title="data-flow" subtitle="Claude / Codex の transcript パースから集約まで" >}}
{{< /cards >}}

## セットアップ

{{< cards >}}
  {{< card link="/setup/local" title="local" subtitle="ローカルマシンへの導入（CLI・hook 登録・Grafana 設定）" >}}
  {{< card link="/setup/server" title="server" subtitle="サーバ送信のセットアップ（オプトイン、k8s 参考デプロイ）" >}}
{{< /cards >}}

## reference 系ドキュメント（GitHub）

- [docs/spec.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md) — 外部契約（CLI・hook 仕様・データモデル）
- [docs/metrics.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/metrics.md) — 計測フレームワーク
- [docs/design.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/design.md) — 実装方針と設計判断
