# セットアップガイド

hitl-metrics を導入する手順です。動作の仕組みや日常の運用については [usage.md](usage.md) を参照してください。

## 前提条件

| ツール | 用途 |
|--------|------|
| Grafana 11+ | ダッシュボード表示 |
| [frser-sqlite-datasource](https://github.com/fr-ser/grafana-sqlite-datasource) | Grafana の SQLite プラグイン |
| gh CLI | PR URL の自動補完（`backfill` コマンド） |
| Docker（任意） | E2E テスト用の Grafana 環境 |

## 1. CLI のインストール

[GitHub Releases](https://github.com/ishii1648/hitl-metrics/releases/latest) から OS/アーキテクチャに合ったアーカイブをダウンロードして展開します。

```fish
# macOS (Apple Silicon) の例
curl -L https://github.com/ishii1648/hitl-metrics/releases/latest/download/hitl-metrics_darwin_arm64.tar.gz | tar xz
mv hitl-metrics ~/.local/bin/
```

`~/.local/bin` が `$PATH` に含まれていることを確認してください。

> **ソースからビルドする場合（開発者向け）**
> ```fish
> git clone https://github.com/ishii1648/hitl-metrics.git
> cd hitl-metrics
> go build -o ~/.local/bin/hitl-metrics ./cmd/hitl-metrics/
> ```

> **`hitl-metrics setup` と `make install` の違い**
>
> - `make install` … バイナリ自体を `$PREFIX/bin` に配置する（`go build`）。
> - `hitl-metrics setup` … hook 登録の **手順を表示** するだけで、ファイルは書きません。

## 2. hook の登録

hitl-metrics が利用する hook は **dotfiles または手動** で登録します。`hitl-metrics setup` は登録例を表示するだけで自動登録はしません（dotfiles 等で settings.json / config.toml を一元管理する構成と整合させるため）。

```fish
hitl-metrics setup                # 両 agent の登録例を表示
hitl-metrics setup --agent claude
hitl-metrics setup --agent codex
```

### Claude Code (`~/.claude/settings.json`)

```json
{
  "hooks": {
    "SessionStart": [
      {"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-start --agent claude"}]},
      {"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook todo-cleanup"}]}
    ],
    "SessionEnd": [
      {"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook session-end --agent claude", "timeout": 10}]}
    ],
    "Stop": [
      {"matcher": "", "hooks": [{"type": "command", "command": "hitl-metrics hook stop --agent claude"}]}
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
      {"hooks": [{"type": "command", "command": "hitl-metrics hook session-start --agent codex"}]}
    ],
    "Stop": [
      {"hooks": [{"type": "command", "command": "hitl-metrics hook stop --agent codex"}]}
    ],
    "PostToolUse": [
      {"hooks": [{"type": "command", "command": "hitl-metrics hook post-tool-use --agent codex"}]}
    ]
  }
}
```

`config.toml` 形式で書く場合は `[features] codex_hooks = true` を有効にした上で `[[hooks.SessionStart]]` / `[[hooks.Stop]]` を追加します。

### 検証

```fish
hitl-metrics doctor
```

binary の PATH 配置・データディレクトリ（`~/.claude/`, `~/.codex/`）の存在・hook 登録状況を agent ごとにチェックします。未登録の hook は warning として表示しますが、**自動修復は行いません**（dotfiles 一元管理の前提を壊さないため）。

> **過去に `hitl-metrics install` で自動登録した hook を取り除きたい場合**
>
> ```fish
> hitl-metrics uninstall-hooks
> ```
>
> 旧バージョンが書き込んだ `~/.claude/settings.json` の単一フックエントリのみを削除します。matcher 付きエントリや複数フックを束ねたエントリは（人間が編集した可能性が高いため）触りません。Codex 側 (`~/.codex/config.toml`) は人間編集が前提のため自動削除を提供しません。

## 3. 初回データ生成

```fish
hitl-metrics backfill
hitl-metrics sync-db
```

`~/.claude/hitl-metrics.db` が生成されます（DB は両 agent を集約します。後方互換のためファイル位置は `~/.claude/` 直下のままです）。以降はセッション終了時に Stop hook が自動実行します。

特定 agent だけを処理したい場合は `--agent <claude|codex>` を付けます。省略時は検出された agent すべてを対象にします。

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
