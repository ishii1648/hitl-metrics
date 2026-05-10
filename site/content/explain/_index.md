---
title: 仕組み解説
weight: 10
bookCollapseSection: false
---

# 仕組み解説

agent-telemetry の動作原理を visualize 込みで解説します。読み順は次を推奨します。

1. [architecture]({{< relref "/explain/architecture" >}}) — 全体像（hook → JSONL → SQLite → Grafana）
2. [data-flow]({{< relref "/explain/data-flow" >}}) — Claude / Codex 各々の transcript パースから集約まで
3. [hooks]({{< relref "/explain/hooks" >}}) — どの hook がどのデータを何に使うか
4. [dashboard]({{< relref "/explain/dashboard" >}}) — Grafana dashboard の panel ごとの読み方
