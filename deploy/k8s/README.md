# agent-telemetry-server — Kubernetes manifests

Kustomize layout for deploying `agent-telemetry-server` plus a colocated Grafana that reads the shared SQLite DB written by the ingest API.

## Layout

```
deploy/k8s/
  base/                          # Deployment / Service / PVC / Secret / ConfigMaps
  overlays/local/                # kind / minikube — NodePort + hostPath PV
  overlays/production/           # Ingress + StorageClass + resources
```

The Grafana ConfigMaps are generated from the canonical files under `grafana/` at the repo root, so dashboard / datasource changes flow through to both `make grafana-up` (docker-compose) and `kubectl apply -k …` (k8s) without duplication.

## Apply

The kustomization references files outside its directory (`grafana/`, two levels up). Pass `--load-restrictor=LoadRestrictionsNone` so Kustomize will follow those paths:

```fish
# kind / minikube smoke test
kubectl apply -k deploy/k8s/overlays/local --load-restrictor=LoadRestrictionsNone

# production (edit overlays/production/ingress.yaml host first)
kubectl apply -k deploy/k8s/overlays/production --load-restrictor=LoadRestrictionsNone
```

To preview the rendered manifest without applying:

```fish
kubectl kustomize --load-restrictor=LoadRestrictionsNone deploy/k8s/overlays/local
```

## Local overlay quickstart

```fish
kind create cluster
kubectl apply -k deploy/k8s/overlays/local --load-restrictor=LoadRestrictionsNone

# server NodePort 30843, Grafana NodePort 30300
kubectl -n agent-telemetry get svc
```

The local overlay installs a sample token (`local-dev-token`) — replace it before exposing the cluster:

```fish
kubectl -n agent-telemetry create secret generic agent-telemetry-server-token \
    --from-literal=token=$(openssl rand -hex 32) \
    --dry-run=client -o yaml | kubectl apply -f -
```

Clients put the same value under `[server] token` in `~/.claude/agent-telemetry.toml`.

## Production notes

- Replace the `host:` values in `overlays/production/ingress.yaml`, set the cert-manager `ClusterIssuer`, and adjust `ingressClassName`.
- Replace `storageClassName: standard` in `overlays/production/pvc-storageclass.yaml` with whatever your cluster provides.
- Use SealedSecrets / ExternalSecrets for the API token. The `secretGenerator` stub in `kustomization.yaml` exists so the manifest applies cleanly when overridden by an operator.
- `replicas: 1` is hard-coded for the server and Grafana — both write SQLite on the shared PVC and would corrupt it under multi-replica writes. If you need HA later, switch to a different storage engine first.

## Schema version coupling

Server and clients must run with matching `internal/syncdb/schema/schema.sql` hashes. Roll out new schemas by:

1. Deploying the new server image (re-applies DDL on startup).
2. Updating client binaries.
3. Running `agent-telemetry sync-db --recheck && agent-telemetry push --full` on each client.

Mid-rollout, clients still on the old hash receive `{"schema_mismatch": true}` and stop ingesting.
