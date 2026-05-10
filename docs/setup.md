# セットアップガイド

agent-telemetry を導入する手順です。動作の仕組みや日常の運用については [usage.md](usage.md) を参照してください。

## 前提条件

| ツール | 用途 |
|--------|------|
| Grafana 11+ | ダッシュボード表示 |
| [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource) | Grafana の SQLite プラグイン |
| gh CLI | PR URL の自動補完（`backfill` コマンド） |
| Docker（任意） | E2E テスト用の Grafana 環境 |

## 1. CLI のインストール

[GitHub Releases](https://github.com/ishii1648/agent-telemetry/releases/latest) から OS/アーキテクチャに合ったアーカイブをダウンロードして展開します。

```fish
# macOS (Apple Silicon) の例
curl -L https://github.com/ishii1648/agent-telemetry/releases/latest/download/agent-telemetry_darwin_arm64.tar.gz | tar xz
mv agent-telemetry ~/.local/bin/
```

`~/.local/bin` が `$PATH` に含まれていることを確認してください。

> **ソースからビルドする場合（開発者向け）**
> ```fish
> git clone https://github.com/ishii1648/agent-telemetry.git
> cd agent-telemetry
> go build -o ~/.local/bin/agent-telemetry ./cmd/agent-telemetry/
> ```

> **`agent-telemetry setup` と `make install` の違い**
>
> - `make install` … バイナリ自体を `$PREFIX/bin` に配置する（`go build`）。
> - `agent-telemetry setup` … hook 登録の **手順を表示** するだけで、ファイルは書きません。

## 2. hook の登録

agent-telemetry が利用する hook は **dotfiles または手動** で登録します。`agent-telemetry setup` は登録例を表示するだけで自動登録はしません（dotfiles 等で settings.json / config.toml を一元管理する構成と整合させるため）。

```fish
agent-telemetry setup                # 両 agent の登録例を表示
agent-telemetry setup --agent claude
agent-telemetry setup --agent codex
```

### Claude Code (`~/.claude/settings.json`)

```json
{
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "agent-telemetry hook session-start --agent claude"}]}
    ],
    "SessionEnd": [
      {"matcher": "", "hooks": [{"type": "command", "command": "agent-telemetry hook session-end --agent claude", "timeout": 10}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "agent-telemetry hook stop --agent claude"}]}
    ]
  }
}
```

`--agent` を省略しても既定値が `claude` のため動作します。

### Codex CLI (`~/.codex/hooks.json` または `~/.codex/config.toml`)

Codex には `SessionEnd` イベントが存在しないため、`Stop` hook が SessionEnd を兼ねます（最後の Stop 発火が事実上の SessionEnd）。`PostToolUse` hook は任意で、`gh pr create` 等の出力から PR URL を session-index に追記します。

```json
{
  "hooks": {
    "SessionStart": [
      {"hooks": [{"type": "command", "command": "agent-telemetry hook session-start --agent codex"}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "agent-telemetry hook stop --agent codex"}]}
    ],
    "PostToolUse": [
      {"hooks": [{"type": "command", "command": "agent-telemetry hook post-tool-use --agent codex"}]}
    ]
  }
}
```

`config.toml` 形式で書く場合は `[features] codex_hooks = true` を有効にした上で `[[hooks.SessionStart]]` / `[[hooks.Stop]]` を追加します。

### 検証

```fish
agent-telemetry doctor
```

binary の PATH 配置・データディレクトリ（`~/.claude/`, `~/.codex/`）の存在・hook 登録状況を agent ごとにチェックします。未登録の hook は warning として表示しますが、**自動修復は行いません**（dotfiles 一元管理の前提を壊さないため）。

> **過去に `agent-telemetry install` で自動登録した hook を取り除きたい場合**
>
> ```fish
> agent-telemetry uninstall-hooks
> ```
>
> 旧バージョンが書き込んだ `~/.claude/settings.json` の単一フックエントリのみを削除します。matcher 付きエントリや複数フックを束ねたエントリは（人間が編集した可能性が高いため）触りません。Codex 側 (`~/.codex/config.toml`) は人間編集が前提のため自動削除を提供しません。

## 3. 初回データ生成

```fish
agent-telemetry backfill
agent-telemetry sync-db
```

`~/.claude/agent-telemetry.db` が生成されます（DB は両 agent を集約します。後方互換のためファイル位置は `~/.claude/` 直下のままです）。以降はセッション終了時に Stop hook が自動実行します。

特定 agent だけを処理したい場合は `--agent <claude|codex>` を付けます。省略時は検出された agent すべてを対象にします。

## 4. Grafana ダッシュボードの設定

### 方法 A: ローカル Grafana に手動設定

1. Grafana に [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource) プラグインをインストール

2. データソースを追加
   - Type: `SQLite`
   - Path: `~/.claude/agent-telemetry.db`（フルパスで指定）

3. ダッシュボードをインポート
   - Grafana の Import 画面で `grafana/dashboards/agent-telemetry.json` をアップロード
   - データソースに上記で作成した SQLite データソースを選択

### 方法 B: プロビジョニングファイルで自動設定

Grafana の設定ディレクトリにプロビジョニングファイルを配置します。

```fish
# データソース設定をコピー（パスを環境に合わせて編集）
cp grafana/provisioning/datasources/agent-telemetry.yaml /etc/grafana/provisioning/datasources/

# ダッシュボード設定をコピー
cp grafana/provisioning/dashboards/agent-telemetry.yaml /etc/grafana/provisioning/dashboards/

# ダッシュボード JSON をコピー
cp -r grafana/dashboards /var/lib/grafana/dashboards/agent-telemetry
```

データソース設定の `path` を自分の環境に合わせて変更してください。

```yaml
# grafana/provisioning/datasources/agent-telemetry.yaml
jsonData:
  path: /Users/<your-username>/.claude/agent-telemetry.db
```

## 5. サーバ送信を有効化する（オプトイン）

複数マシンやチームメンバーで集計値を集約したい場合、`agent-telemetry-server` を立てて `agent-telemetry push` で送信する経路を有効化できます。設定しなければローカル単独利用は従来どおり動きます。仕様の外部契約は [docs/spec.md ## サーバ送信](spec.md#サーバ送信)、設計判断は [docs/design.md ## サーバ側集約パイプライン](design.md#サーバ側集約パイプライン) を参照してください。

agent-telemetry が公式に配布するのは **container image** と **Go binary** のみです:

| 配布物 | 入手元 |
|---|---|
| Container image | `ghcr.io/ishii1648/agent-telemetry-server`（multi-arch: linux/amd64 + arm64） |
| Go binary | [GitHub Releases](https://github.com/ishii1648/agent-telemetry/releases/latest) の `agent-telemetry-server_*` archive |

Helm / Argo CD / Flux / 素の kubectl といった **デプロイ手段** と、StorageClass / IngressClass / cert-manager の有無といった **cluster topology** は運用者の責務です。本節では `kubectl apply -f -` できる粒度の **参考 YAML スニペット** を 2 種類示すので、自分の cluster に合わせて改変してください。

### 5.1 image を取得する

```fish
docker pull ghcr.io/ishii1648/agent-telemetry-server:latest
# 本番では tag pin (vX.Y.Z) を推奨
docker pull ghcr.io/ishii1648/agent-telemetry-server:v0.6.0
```

ローカル動作確認は `docker run` で完結します:

```fish
docker run --rm -p 8443:8443 \
  -e AGENT_TELEMETRY_SERVER_TOKEN=(openssl rand -hex 32) \
  -v $PWD/agent-telemetry-data:/var/lib/agent-telemetry \
  ghcr.io/ishii1648/agent-telemetry-server:latest
```

### 5.2 Bearer token を生成する

`AGENT_TELEMETRY_SERVER_TOKEN` はサーバ起動時の Bearer 認証 key です。クライアント `~/.claude/agent-telemetry.toml` の `[server] token` と同値にする必要があります。

```fish
openssl rand -hex 32
```

漏えい時はサーバ側 env と全クライアントの `[server] token` を同時にローテーションしてください。

### 5.3 k8s 参考デプロイ — 最小構成（サーバのみ）

サーバ単体を立て、Grafana は既存環境を使う構成です。下記スニペットを `kubectl apply -f -` してください。`# REPLACE_ME` コメントの箇所は cluster ごとに調整します。

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: agent-telemetry
---
apiVersion: v1
kind: Secret
metadata:
  name: agent-telemetry-server-token
  namespace: agent-telemetry
type: Opaque
stringData:
  # REPLACE_ME: openssl rand -hex 32 で生成し、クライアント [server] token と同値にする。
  # SealedSecret / External Secrets Operator など秘匿配信に置き換えるのが望ましい。
  token: "REPLACE_ME_with_openssl_rand_hex_32"
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: agent-telemetry-data
  namespace: agent-telemetry
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: REPLACE_ME   # REPLACE_ME: cluster の StorageClass 名（`kubectl get storageclass`）
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-telemetry-server
  namespace: agent-telemetry
spec:
  replicas: 1                    # SQLite なので 1 固定。スケールアウトは未対応
  strategy:
    type: Recreate               # WAL を複数 pod が同時に開かないため RollingUpdate ではなく Recreate
  selector:
    matchLabels: {app: agent-telemetry-server}
  template:
    metadata:
      labels: {app: agent-telemetry-server}
    spec:
      containers:
        - name: server
          image: ghcr.io/ishii1648/agent-telemetry-server:latest   # REPLACE_ME: 本番では vX.Y.Z で tag pin
          args: ["--listen", ":8443", "--data-dir", "/var/lib/agent-telemetry"]
          ports:
            - {containerPort: 8443, name: ingest}
          env:
            - name: AGENT_TELEMETRY_SERVER_TOKEN
              valueFrom:
                secretKeyRef:
                  name: agent-telemetry-server-token
                  key: token
          volumeMounts:
            - {name: data, mountPath: /var/lib/agent-telemetry}
          readinessProbe:
            httpGet: {path: /healthz, port: ingest}
          resources:
            requests: {cpu: "50m",  memory: "64Mi"}
            limits:   {cpu: "500m", memory: "256Mi"}
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: agent-telemetry-data
---
apiVersion: v1
kind: Service
metadata:
  name: agent-telemetry-server
  namespace: agent-telemetry
spec:
  type: ClusterIP
  selector: {app: agent-telemetry-server}
  ports:
    - {port: 8443, targetPort: ingest, name: ingest}
---
# REPLACE_ME: 外部公開する場合の Ingress 例。cluster の IngressClass / cert-manager Issuer に合わせて調整。
# 公開しない場合（VPN / port-forward 経由のみ）はこのブロックをまるごと削除してよい。
# apiVersion: networking.k8s.io/v1
# kind: Ingress
# metadata:
#   name: agent-telemetry-server
#   namespace: agent-telemetry
#   annotations:
#     cert-manager.io/cluster-issuer: REPLACE_ME       # 例: letsencrypt-prod
# spec:
#   ingressClassName: REPLACE_ME                       # 例: nginx
#   tls:
#     - hosts: [telemetry.example.com]
#       secretName: agent-telemetry-tls
#   rules:
#     - host: telemetry.example.com
#       http:
#         paths:
#           - path: /
#             pathType: Prefix
#             backend:
#               service:
#                 name: agent-telemetry-server
#                 port: {number: 8443}
```

### 5.4 k8s 参考デプロイ — Grafana 同居版

サーバと Grafana を **同 pod の sidecar** として配置することで、`ReadWriteOnce` PVC のまま両者で同じ SQLite を共有できます。Grafana の datasource provisioning yaml は `grafana/provisioning/datasources/agent-telemetry-docker.yaml` を **そのまま** ConfigMap として配るので、ローカル `make grafana-up` と完全に同じダッシュボードが描画されます。

ConfigMap はリポジトリのファイルから生成します:

```fish
# datasource + dashboards provisioning
kubectl create configmap agent-telemetry-grafana-provisioning -n agent-telemetry \
  --from-file=datasources.yaml=grafana/provisioning/datasources/agent-telemetry-docker.yaml \
  --from-file=dashboards.yaml=grafana/provisioning/dashboards/agent-telemetry-docker.yaml \
  --dry-run=client -o yaml | kubectl apply -f -

# dashboard JSON 本体
kubectl create configmap agent-telemetry-grafana-dashboards -n agent-telemetry \
  --from-file=agent-telemetry.json=grafana/dashboards/agent-telemetry.json \
  --dry-run=client -o yaml | kubectl apply -f -
```

> **ConfigMap サイズ上限**: ConfigMap は etcd の制約から 1 MiB が上限です。dashboard JSON が肥大化した場合は、Grafana sidecar pattern（[grafana/helm-charts](https://github.com/grafana/helm-charts) の sidecar dashboards loader）または initContainer で `git clone` する形に切り替えてください。

そのうえで以下を `kubectl apply -f -` します。Secret / PVC / Service の最小構成は 5.3 と共通なので、ここでは Deployment + Service だけを示します（5.3 の Secret + PVC + Namespace は事前に apply 済みである前提）。

```yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-telemetry
  namespace: agent-telemetry
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels: {app: agent-telemetry}
  template:
    metadata:
      labels: {app: agent-telemetry}
    spec:
      containers:
        - name: server
          image: ghcr.io/ishii1648/agent-telemetry-server:latest   # REPLACE_ME: 本番では tag pin
          args: ["--listen", ":8443", "--data-dir", "/var/lib/agent-telemetry"]
          ports:
            - {containerPort: 8443, name: ingest}
          env:
            - name: AGENT_TELEMETRY_SERVER_TOKEN
              valueFrom:
                secretKeyRef:
                  name: agent-telemetry-server-token
                  key: token
          volumeMounts:
            - {name: data, mountPath: /var/lib/agent-telemetry}
          readinessProbe:
            httpGet: {path: /healthz, port: ingest}

        - name: grafana
          image: grafana/grafana-oss:11.5.2
          ports:
            - {containerPort: 3000, name: http}
          env:
            - {name: GF_INSTALL_PLUGINS,         value: "frser-sqlite-datasource"}
            - {name: GF_AUTH_ANONYMOUS_ENABLED,  value: "true"}
            - {name: GF_AUTH_ANONYMOUS_ORG_ROLE, value: "Viewer"}
          volumeMounts:
            # 同じ PVC を別 path で mount。サーバが /var/lib/agent-telemetry/agent-telemetry.db に書いた
            # SQLite が、Grafana 側からは /var/lib/grafana/agent-telemetry.db として見える
            # （docker-compose と同じ datasource yaml をそのまま流用するため）。
            # Grafana 自身の state（grafana.db / plugins / png）も同 PVC root に並ぶが副作用なし。
            - {name: data, mountPath: /var/lib/grafana}
            - name: provisioning
              mountPath: /etc/grafana/provisioning/datasources/agent-telemetry.yaml
              subPath: datasources.yaml
            - name: provisioning
              mountPath: /etc/grafana/provisioning/dashboards/agent-telemetry.yaml
              subPath: dashboards.yaml
            - {name: dashboards, mountPath: /var/lib/grafana/dashboards}
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: agent-telemetry-data
        - name: provisioning
          configMap:
            name: agent-telemetry-grafana-provisioning
        - name: dashboards
          configMap:
            name: agent-telemetry-grafana-dashboards
---
apiVersion: v1
kind: Service
metadata:
  name: agent-telemetry
  namespace: agent-telemetry
spec:
  type: ClusterIP
  selector: {app: agent-telemetry}
  ports:
    - {port: 8443, targetPort: ingest, name: ingest}
    - {port: 3000, targetPort: http,   name: grafana}
```

Grafana にブラウザでアクセスする手順は [usage.md ## サーバ DB を Grafana で見る](usage.md#サーバ-db-を-grafana-で見る) を参照してください。

### 5.5 クライアント設定

`~/.claude/agent-telemetry.toml` に `[server]` セクションを追加します:

```toml
user = "you@example.com"

[server]
endpoint = "https://telemetry.example.com"   # サーバの base URL（パスは含めない）
token    = "xxx"                             # AGENT_TELEMETRY_SERVER_TOKEN と同値
```

設定を確認:

```fish
agent-telemetry push --dry-run        # 対象セッション件数と payload サイズだけ表示
agent-telemetry push --since-last     # 実送信。差分のみ
```

`[server]` が欠落 / 値が空のときは warning を stderr に出して exit code 0 で終了するため、cron に設定したまま config を取り除いても CI / cron が壊れません。

### 5.6 push の定期起動

`agent-telemetry push --since-last` は Stop hook の hot path に乗せず、別途定期起動します（hook が遅延すると agent UX が劣化するため）。exit code は **0 = ok / 1 = error / 2 = schema_mismatch** です。

#### cron（Linux / macOS）

```cron
*/5 * * * * /usr/local/bin/agent-telemetry push --since-last >> $HOME/.claude/logs/push.log 2>&1
```

#### launchd plist（macOS）

`~/Library/LaunchAgents/dev.agent-telemetry.push.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>dev.agent-telemetry.push</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/agent-telemetry</string>
    <string>push</string>
    <string>--since-last</string>
  </array>
  <key>StartInterval</key>
  <integer>300</integer>
  <key>StandardOutPath</key>
  <string>/Users/REPLACE_ME/.claude/logs/push.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/REPLACE_ME/.claude/logs/push.log</string>
</dict>
</plist>
```

```fish
launchctl load ~/Library/LaunchAgents/dev.agent-telemetry.push.plist
```

### 5.7 新メトリクス追加時の遡及反映

サーバ・クライアント間で `internal/syncdb/schema.sql` のハッシュ（`schema_meta`）が一致している必要があります。新メトリクスを追加する場合は **サーバを先に新スキーマへ更新** します（クライアント先行で push されると古いスキーマで永続化されるため）。

```fish
# 1. サーバを新 image にロールアウト
kubectl set image deployment/agent-telemetry-server \
  server=ghcr.io/ishii1648/agent-telemetry-server:v0.6.0 -n agent-telemetry
kubectl rollout status deployment/agent-telemetry-server -n agent-telemetry

# 2. 全クライアントで binary を更新（Releases から再 install）

# 3. 各クライアントで過去全セッションを再集計し再送信
agent-telemetry sync-db --recheck
agent-telemetry push --full
```

#### `schema_mismatch` エラーが出たとき

クライアントが古いまま push するとサーバが `schema_mismatch: true` を返し、クライアントは **exit code 2** で終了します。

```
$ agent-telemetry push --since-last
schema mismatch: client=abc123… server=def456… — upgrade client binary
$ echo $status
2
```

対処は単純に **クライアント binary を新 version へ更新する** だけです:

```fish
agent-telemetry version    # 現在の binary version を確認
# Releases から最新を再 install
agent-telemetry sync-db --recheck && agent-telemetry push --full
```

サーバを過去 version にロールバックする場合も同じ手順で **クライアント側を先に旧 version に戻し**、その後サーバ image を差し替えます。

## 6. hitl-metrics（旧名）からの移行

`hitl-metrics` を使っていた環境からは以下の手順で移行します。背景は [issues/closed/0021-spec-rename-hitl-metrics-to-agent-telemetry.md](../issues/closed/0021-spec-rename-hitl-metrics-to-agent-telemetry.md) を参照。

### 6.1 バイナリの差し替え

旧 `hitl-metrics` バイナリは PATH から取り除いてから新 `agent-telemetry` を配置します。両方が PATH 上に共存すると、settings.json の hook entry が古いバイナリを呼び続ける事故が起きやすいため。

```fish
# 旧バイナリの場所を確認
which hitl-metrics

# 削除（自分の install 場所に合わせて調整）
rm ~/.local/bin/hitl-metrics
```

`agent-telemetry upgrade` 実行時にも旧バイナリが PATH にあれば warning を出します。

### 6.2 DB / state ファイルの自動移行

`agent-telemetry backfill` または `agent-telemetry sync-db` を実行すると以下のファイルを自動でリネームします。

| 旧 | 新 |
|---|---|
| `~/.claude/hitl-metrics.db` (+ `-wal`, `-shm`) | `~/.claude/agent-telemetry.db` (+ `-wal`, `-shm`) |
| `~/.claude/hitl-metrics-state.json` | `~/.claude/agent-telemetry-state.json` |
| `~/.codex/hitl-metrics-state.json` | `~/.codex/agent-telemetry-state.json` |

新旧両方が存在する場合は安全のためリネームを中止し、stderr に warning を出します。手動でいずれか一方を削除してから再実行してください。

一括で移行したい場合は `scripts/migrate-db-name.sh` を使えます（CLI 実行と等価）。

### 6.3 hook 設定の更新

`~/.claude/settings.json` / `~/.codex/hooks.json` の hook command を `hitl-metrics hook ...` から `agent-telemetry hook ...` に書き換えます。`agent-telemetry doctor` は旧名のまま登録された hook を warning として一覧表示します。

旧 `hitl-metrics install` で自動登録された単一エントリは `agent-telemetry uninstall-hooks` で削除できます（旧名 / 新名どちらの command 文字列でもマッチします）。

### 6.4 Grafana

ダッシュボード `uid` / datasource `uid` を `hitl-metrics` から `agent-telemetry` に切り替えています。Grafana provisioning を使っている場合は本リポジトリの `grafana/provisioning/` を再配備してください。手動で datasource を作成していた場合は `Path` を新 DB ファイル名に変更します。

旧ダッシュボード（`uid: hitl-metrics`）は Grafana 側に残ったままになります。不要なら Grafana UI から削除してください（自動削除は行いません）。
