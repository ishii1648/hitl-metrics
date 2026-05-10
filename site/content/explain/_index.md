---
title: 仕組み解説
weight: 10
sidebar:
  open: true
---

# 仕組み解説

agent-telemetry の動作原理を visualize 込みで解説します。読み順は次を推奨します。

1. [metrics](metrics/) — 何を観察しているか・なぜそれを選んだか（観察軸と落とし穴）
2. [architecture](architecture/) — 全体像（hook → JSONL → SQLite → Grafana）
3. [hooks](hooks/) — どの hook がどのデータを何に使うか
4. [data-flow](data-flow/) — Claude / Codex 各々の transcript パースから集約まで
5. [dashboard](dashboard/) — Grafana dashboard の panel ごとの読み方
