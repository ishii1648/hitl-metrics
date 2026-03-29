# セットアップガイド

hitl-metrics を導入する手順です。動作の仕組みや日常の運用については [usage.md](usage.md) を参照してください。

## 前提条件

| ツール | 用途 |
|--------|------|
| Go 1.22+ | CLI ビルド |
| Grafana 11+ | ダッシュボード表示 |
| [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource) | Grafana の SQLite プラグイン |
| gh CLI | PR URL の自動補完（`backfill` コマンド） |
| Docker（任意） | E2E テスト用の Grafana 環境 |

## 1. CLI のビルド

```fish
git clone https://github.com/ishii1648/hitl-metrics.git
cd hitl-metrics
go build -o ~/.local/bin/hitl-metrics ./cmd/hitl-metrics/
```

`~/.local/bin` が `$PATH` に含まれていることを確認してください。

## 2. Claude Code hook の登録

```fish
hitl-metrics install
```

`~/.claude/settings.json` に hook が自動登録されます。既に登録済みの hook はスキップされます。

<details>
<summary>手動で設定する場合</summary>

`~/.claude/settings.json` に以下を追加します。`/path/to/hitl-metrics` はクローン先のパスに置き換えてください。

```json
{
  "hooks": {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/hitl-metrics/hooks/session-index.sh"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/hitl-metrics/hooks/permission-log.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/hitl-metrics/hooks/stop.sh"
          }
        ]
      }
    ]
  }
}
```
</details>

## 3. 初回データ生成

```fish
hitl-metrics backfill
hitl-metrics sync-db
```

`~/.claude/hitl-metrics.db` が生成されます。以降はセッション終了時に Stop hook が自動実行します。

## 4. Grafana ダッシュボードの設定

### 方法 A: ローカル Grafana に手動設定

1. Grafana に [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource) プラグインをインストール

2. データソースを追加
   - Type: `SQLite`
   - Path: `~/.claude/hitl-metrics.db`（フルパスで指定）

3. ダッシュボードをインポート
   - Grafana の Import 画面で `grafana/dashboards/hitl-metrics.json` をアップロード
   - データソースに上記で作成した SQLite データソースを選択

### 方法 B: プロビジョニングファイルで自動設定

Grafana の設定ディレクトリにプロビジョニングファイルを配置します。

```fish
# データソース設定をコピー（パスを環境に合わせて編集）
cp grafana/provisioning/datasources/hitl-metrics.yaml /etc/grafana/provisioning/datasources/

# ダッシュボード設定をコピー
cp grafana/provisioning/dashboards/hitl-metrics.yaml /etc/grafana/provisioning/dashboards/

# ダッシュボード JSON をコピー
cp -r grafana/dashboards /var/lib/grafana/dashboards/hitl-metrics
```

データソース設定の `path` を自分の環境に合わせて変更してください。

```yaml
# grafana/provisioning/datasources/hitl-metrics.yaml
jsonData:
  path: /Users/<your-username>/.claude/hitl-metrics.db
```
