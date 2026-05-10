---
title: セットアップ
weight: 20
sidebar:
  open: true
---

agent-telemetry のセットアップと運用手順をまとめます。仕組みの理解から先に始めたい場合は [仕組み解説]({{< relref "/explain" >}}) を先に読むと、各セットアップ手順が「どこに効いているか」を把握しやすくなります。

1. [install]({{< relref "/setup/install" >}}) — CLI のインストールと hook 登録、Grafana ダッシュボード設定
2. [server]({{< relref "/setup/server" >}}) — `agent-telemetry-server` を立てて複数マシン / チームで集計値を集約する（オプトイン）
3. [usage]({{< relref "/setup/usage" >}}) — 日常運用、Grafana の起動、自動化サンプル、トラブルシューティング
