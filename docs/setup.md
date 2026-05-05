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
      {"matcher": "", "hooks": [{"type": "command", "command": "agent-telemetry hook session-start --agent claude"}]},
      {"matcher": "", "hooks": [{"type": "command", "command": "agent-telemetry hook todo-cleanup"}]}
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

## 5. hitl-metrics（旧名）からの移行

`hitl-metrics` を使っていた環境からは以下の手順で移行します。背景は [history.md](history.md) の「8. リポジトリ名変更」を参照。

### 5.1 バイナリの差し替え

旧 `hitl-metrics` バイナリは PATH から取り除いてから新 `agent-telemetry` を配置します。両方が PATH 上に共存すると、settings.json の hook entry が古いバイナリを呼び続ける事故が起きやすいため。

```fish
# 旧バイナリの場所を確認
which hitl-metrics

# 削除（自分の install 場所に合わせて調整）
rm ~/.local/bin/hitl-metrics
```

`agent-telemetry upgrade` 実行時にも旧バイナリが PATH にあれば warning を出します。

### 5.2 DB / state ファイルの自動移行

`agent-telemetry backfill` または `agent-telemetry sync-db` を実行すると以下のファイルを自動でリネームします。

| 旧 | 新 |
|---|---|
| `~/.claude/hitl-metrics.db` (+ `-wal`, `-shm`) | `~/.claude/agent-telemetry.db` (+ `-wal`, `-shm`) |
| `~/.claude/hitl-metrics-state.json` | `~/.claude/agent-telemetry-state.json` |
| `~/.codex/hitl-metrics-state.json` | `~/.codex/agent-telemetry-state.json` |

新旧両方が存在する場合は安全のためリネームを中止し、stderr に warning を出します。手動でいずれか一方を削除してから再実行してください。

一括で移行したい場合は `scripts/migrate-db-name.sh` を使えます（CLI 実行と等価）。

### 5.3 hook 設定の更新

`~/.claude/settings.json` / `~/.codex/hooks.json` の hook command を `hitl-metrics hook ...` から `agent-telemetry hook ...` に書き換えます。`agent-telemetry doctor` は旧名のまま登録された hook を warning として一覧表示します。

旧 `hitl-metrics install` で自動登録された単一エントリは `agent-telemetry uninstall-hooks` で削除できます（旧名 / 新名どちらの command 文字列でもマッチします）。

### 5.4 Grafana

ダッシュボード `uid` / datasource `uid` を `hitl-metrics` から `agent-telemetry` に切り替えています。Grafana provisioning を使っている場合は本リポジトリの `grafana/provisioning/` を再配備してください。手動で datasource を作成していた場合は `Path` を新 DB ファイル名に変更します。

旧ダッシュボード（`uid: hitl-metrics`）は Grafana 側に残ったままになります。不要なら Grafana UI から削除してください（自動削除は行いません）。
