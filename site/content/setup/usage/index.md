---
title: usage
weight: 30
---

agent-telemetry の日常運用と Grafana 起動・自動化サンプル・トラブルシューティングをまとめます。セットアップ手順は [install]({{< relref "/setup/install" >}}) を、サーバ送信は [server]({{< relref "/setup/server" >}}) を参照してください。

データフローや hook の役割といった動作の仕組みは [仕組み解説]({{< relref "/explain" >}}) と [docs/spec.md](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md) を参照してください。CLI コマンドの完全な一覧は [docs/spec.md ## CLI](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md#cli) を正本とします。

## 日常の運用

Stop hook が登録済みであれば、セッション終了時に `backfill` と `sync-db` が自動実行されます。手動で即時更新する場合:

```fish
agent-telemetry backfill        # 検出された agent すべて
agent-telemetry sync-db         # 検出された agent すべて
agent-telemetry backfill --agent codex
```

ダッシュボードは `http://localhost:3000`（デフォルト）でアクセスできます。

`backfill` / `sync-db` / `doctor` は検出された agent **すべて** を対象にします。明示的に絞り込みたいときだけ `--agent <claude|codex>` を付けます。検出ロジックの詳細は [docs/spec.md ## CLI](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md#cli) を参照してください。

## Grafana を Docker で起動する

実データ（`~/.claude/agent-telemetry.db`）を使った Grafana 環境:

```fish
make grafana-up          # 実 DB を mount して Grafana + Image Renderer 起動 → http://localhost:13000
make grafana-down        # コンテナ停止
```

DB パスを上書きしたい場合は `AGENT_TELEMETRY_DB` を渡す:

```fish
make grafana-up AGENT_TELEMETRY_DB=/custom/path/agent-telemetry.db
```

> **注意**: mount は読み書き可能です（SQLite が WAL モードのため `:ro` mount は不可）。frser-sqlite-datasource は SELECT のみで書き込みは行わないので実害はありませんが、Grafana コンテナに DB ファイルへの書き込み権限が渡る点を留意してください。

### サーバ DB を Grafana で見る

サーバ送信を有効化している場合（[server]({{< relref "/setup/server" >}})）、サーバ側 SQLite (`<data_dir>/agent-telemetry.db`) を 2 通りの経路で Grafana から参照できます。datasource の `uid: agent-telemetry` を踏襲しているため、ローカル `make grafana-up` と **同じダッシュボード JSON** がそのまま動きます。

#### k8s 経由 — 同居版 Grafana を Port-forward

[server]({{< relref "/setup/server" >}}) の Grafana 同居版を deploy 済みなら、Service を Port-forward するだけで開けます:

```fish
kubectl port-forward -n agent-telemetry svc/agent-telemetry 3000:3000
# → http://localhost:3000 で Grafana にアクセス
```

NodePort / Ingress / LoadBalancer で外部公開する場合は cluster の慣習に合わせて `Service.spec.type` を変更してください。

#### ローカル開発時 — サーバ DB ファイルを直接 mount

サーバ DB のスナップショットを手元に持ってきて、ローカル Grafana で見たい場合（個人検証や比較用）。`AGENT_TELEMETRY_DB` を server data dir 内のファイルに向ければ、`make grafana-up` がそのまま動きます:

```fish
# サーバから DB をコピー（k8s の場合の例。VPS / docker 環境ならその慣習で）
kubectl cp -n agent-telemetry agent-telemetry-0:/var/lib/agent-telemetry/agent-telemetry.db /tmp/server-snapshot.db

# ローカル Grafana で開く
make grafana-up AGENT_TELEMETRY_DB=/tmp/server-snapshot.db
# → http://localhost:13000
```

サーバ DB スキーマはクライアント DB と同一なので、ダッシュボードは無調整で描画されます。

#### 送信内容を確認する — `agent-telemetry push --dry-run`

実送信する前に対象セッション件数と payload サイズを確認できます:

```fish
agent-telemetry push --dry-run                # 検出された全 agent
agent-telemetry push --dry-run --agent codex  # agent を絞る
```

`[server]` 設定の token / endpoint が空のときは warning を stderr に出して exit code 0 で終わります（cron に設定したまま config を抜いても CI / cron が壊れないように）。

### E2E テスト環境（開発者向け）

決定的な fixture データを使った Grafana 環境（README 用スクリーンショット生成に使用）:

```fish
make grafana-up-e2e      # fixture 生成 → Grafana 起動
make grafana-screenshot  # 全パネルの PNG を取得
make grafana-down        # コンテナ停止
```

## 自動化サンプル

agent-telemetry 本体の責務は外れ値 PR の示唆まで。その先で「外れ値 PR を coding agent に分析させる」「結果を Issue / Slack に流す」といった自動化を組みたい場合のリファレンス実装を [`examples/`](https://github.com/ishii1648/agent-telemetry/tree/main/examples) に同梱している。

- [`examples/skills/analyze-pr/`](https://github.com/ishii1648/agent-telemetry/blob/main/examples/skills/analyze-pr/SKILL.md) — 外れ値 PR の transcript を読み、token 消費の外れ値要因と改善仮説を Markdown で stdout に出す Claude Code skill
- Claude Action（GitHub Actions）/ Claude Web Routine から呼ぶ例は [`examples/README.md`](https://github.com/ishii1648/agent-telemetry/blob/main/examples/README.md) を参照

`examples/` は **best-effort** 扱い。CI で検証しておらず、`make grafana-screenshot` のような必須作業からも外れる。コピーして自分の環境用に書き換える前提のサンプルとして扱う。

> ⚠️ transcript には機密情報（プロンプト全文・コード断片・ローカルパス等）が含まれる可能性がある。skill / script の出力を外部に送信する前に必ずスコープを確認すること。詳細は [`examples/README.md`](https://github.com/ishii1648/agent-telemetry/blob/main/examples/README.md#privacy-注意) を参照。

## トラブルシューティング

### hook が動作しない

- `agent-telemetry doctor` で binary / data dir / hook 登録状況を agent ごとに一括確認
- 未登録の hook があれば dotfiles または手動で `~/.claude/settings.json` または `~/.codex/hooks.json` に追加（[install ## 2. hook の登録]({{< relref "/setup/install" >}}#2-hook-の登録) 参照）
- デバッグログを確認: `~/.claude/logs/session-index-debug.log` または `~/.codex/logs/session-index-debug.log`

### sync-db でデータが空になる

- `~/.claude/session-index.jsonl` または `~/.codex/session-index.jsonl` が存在し、データが記録されているか確認
- session-index の `transcript` パスが存在するか確認
- Codex の場合、`.jsonl.zst` 圧縮ファイルでも透過解凍されるはずです

### Grafana でデータが表示されない

- データソースの Path が `agent-telemetry.db` のフルパスを指しているか確認
- `agent-telemetry sync-db` を再実行して DB を最新化
- Grafana のデータソース設定で「Test」ボタンを押して接続を確認
- ダッシュボードの `coding_agent` テンプレート変数が `All` になっているか確認
