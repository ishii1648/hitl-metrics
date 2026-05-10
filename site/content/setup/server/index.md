---
title: server
weight: 20
---

複数マシンやチームメンバーで集計値を集約したい場合、`agent-telemetry-server` を立てて `agent-telemetry push` で送信する経路を有効化できます。サーバ送信は **オプトイン** で、設定しなければローカル単独利用は従来どおり動きます。基本のローカルセットアップは [local]({{< relref "/setup/local" >}}) を参照してください。

仕様の外部契約は [docs/spec.md ## サーバ送信](https://github.com/ishii1648/agent-telemetry/blob/main/docs/spec.md#サーバ送信)、設計判断は [docs/design.md ## サーバ側集約パイプライン](https://github.com/ishii1648/agent-telemetry/blob/main/docs/design.md#サーバ側集約パイプライン) を参照。

agent-telemetry が公式に配布するのは **container image** と **Go binary** のみです:

| 配布物 | 入手元 |
|---|---|
| Container image | `ghcr.io/ishii1648/agent-telemetry-server`（multi-arch: linux/amd64 + arm64） |
| Go binary | [GitHub Releases](https://github.com/ishii1648/agent-telemetry/releases/latest) の `agent-telemetry-server_*` archive |

Helm / Argo CD / Flux / 素の kubectl といった **デプロイ手段** と、StorageClass / IngressClass / cert-manager の有無といった **cluster topology** は運用者の責務です。本書では `kubectl apply -f -` できる粒度の **参考 YAML スニペット** を 2 種類示すので、自分の cluster に合わせて改変してください。

## 1. image を取得する

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

## 2. Bearer token を生成する

`AGENT_TELEMETRY_SERVER_TOKEN` はサーバ起動時の Bearer 認証 key です。クライアント `~/.config/agent-telemetry/config.toml` の `[server] token` と同値にする必要があります（旧パス `~/.claude/agent-telemetry.toml` も fallback として読まれますが、新規セットアップでは `~/.config/` 側に置いてください）。

```fish
openssl rand -hex 32
```

漏えい時はサーバ側 env と全クライアントの `[server] token` を同時にローテーションしてください。

## 3. k8s 参考デプロイ — 最小構成（サーバのみ）

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

## 4. k8s 参考デプロイ — Grafana 同居版

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

そのうえで以下を `kubectl apply -f -` します。Secret / PVC / Service の最小構成は § 3 と共通なので、ここでは Deployment + Service だけを示します（§ 3 の Secret + PVC + Namespace は事前に apply 済みである前提）。

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

Grafana にブラウザでアクセスする手順は次節 § 5 を参照してください。

## 5. サーバ DB を Grafana で見る

datasource の `uid: agent-telemetry` を踏襲しているため、ローカル `make grafana-up` と **同じダッシュボード JSON** がそのまま動きます。

### 5.1 同居版 Grafana を Port-forward（§ 4 を deploy 済みの場合）

§ 4 の Grafana 同居版を deploy 済みなら、Service を Port-forward するだけで開けます:

```fish
kubectl port-forward -n agent-telemetry svc/agent-telemetry 3000:3000
# → http://localhost:3000 で Grafana にアクセス
```

NodePort / Ingress / LoadBalancer で外部公開する場合は cluster の慣習に合わせて `Service.spec.type` を変更してください。

### 5.2 サーバ DB ファイルを手元にコピーして見る

サーバ側に Grafana を同居させていない場合や、個人検証 / 比較目的でスナップショットを手元で見たい場合。`AGENT_TELEMETRY_DB` を server data dir 内のファイルに向ければ `make grafana-up` がそのまま動きます:

```fish
# サーバから DB をコピー（k8s の場合の例。VPS / docker 環境ならその慣習で）
kubectl cp -n agent-telemetry agent-telemetry-0:/var/lib/agent-telemetry/agent-telemetry.db /tmp/server-snapshot.db

# ローカル Grafana で開く
make grafana-up AGENT_TELEMETRY_DB=/tmp/server-snapshot.db
# → http://localhost:13000
```

サーバ DB スキーマはクライアント DB と同一なので、ダッシュボードは無調整で描画されます。

## 6. クライアント設定

`~/.config/agent-telemetry/config.toml`（`XDG_CONFIG_HOME` が設定されていれば `$XDG_CONFIG_HOME/agent-telemetry/config.toml`）に `[server]` セクションを追加します。旧バージョンが書き出した `~/.claude/agent-telemetry.toml` も fallback として読まれますが、stderr に migration warning が出るので、可能なら `~/.config/` 側に移動してください:

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

## 7. push の定期起動

`agent-telemetry push --since-last` は Stop hook の hot path に乗せず、別途定期起動します（hook が遅延すると agent UX が劣化するため）。exit code は **0 = ok / 1 = error / 2 = schema_mismatch** です。

### cron（Linux / macOS）

```cron
0 * * * * /usr/local/bin/agent-telemetry push --since-last >> $HOME/.claude/logs/push.log 2>&1
```

### launchd plist（macOS）

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
  <integer>3600</integer>
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

## 8. 新メトリクス追加時の遡及反映

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

### `schema_mismatch` エラーが出たとき

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
