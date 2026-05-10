---
decision_type: spec
affected_paths:
  - internal/serverclient/config.go
  - internal/userid/userid.go
  - cmd/agent-telemetry/main.go
  - internal/setup/
  - internal/doctor/
  - docs/spec.md
  - docs/design.md
  - docs/setup.md
  - docs/usage.md
tags: [config, xdg, migration, agent-agnostic]
---

# 設定ファイルパスを XDG Base Directory に移行する

Created: 2026-05-11

## 概要

agent-telemetry の TOML 設定ファイル（`[user]` キー、`[server]` セクション）の参照先を、現在ハードコードされている `~/.claude/agent-telemetry.toml` から XDG Base Directory 仕様に沿った `~/.config/agent-telemetry/config.toml` に移行する。`XDG_CONFIG_HOME` が設定されていればそれを優先する。旧パスは一定期間 fallback として読み続け、stderr に migration warning を出す。

スコープは **設定ファイル (TOML)** のみ。DB (`~/.claude/agent-telemetry.db`) と state.json (`~/.claude/agent-telemetry-state.json`) も同種の問題を持つが、互換性影響が大きく Grafana mount path も絡むので、本 issue のフォローとして別 issue で扱う。

## 根拠

### 現状

設定ファイルパスは下記 2 箇所に同一実装でハードコードされている（コメントでも明示的に同一前提）。

- `internal/serverclient/config.go:37-40` — `ConfigPath()` が `~/.claude/agent-telemetry.toml` を返す。`[server]` セクション（`endpoint` / `token`）を読む
- `internal/userid/userid.go:59-62` — `ConfigPath()` が同パスを返す。top-level `user` キーを読む

加えて、エラーメッセージ・docs にも同パスの直書きが多数ある。

- `cmd/agent-telemetry/main.go:236` — push 失敗時のエラーメッセージで `~/.claude/agent-telemetry.toml` を直書き
- `internal/serverclient/config_test.go` / `internal/userid/userid_test.go` / `internal/serverclient/push_test.go` — テストで同名ファイルを前提
- `docs/spec.md` / `docs/design.md` / `docs/setup.md` / `docs/usage.md` — ユーザ向け説明で同パスを記載
- `site/content/explain/architecture/index.md` 等の解説 docs にも言及あり

### 問題

1. **設定ディレクトリの責務が不正確**: `~/.claude/` は Anthropic Claude Code の公式設定ディレクトリであり、`settings.json` / `agent-telemetry.db`（agent ごとの transcript 元データ近接配置を意図）とは別の文脈で使われる。サードパーティの **設定** ファイルを `~/.claude/` に置くのは責務的に不整合。
2. **agent-agnostic 設計との矛盾**: agent-telemetry は Claude Code / Codex CLI 双方を観測対象とする agent-agnostic ツール（[0018](closed/0018-spec-multi-coding-agent-support.md) 参照）。にもかかわらず Claude 側のディレクトリにのみ設定ファイルを置くのは Codex ユーザから見て直感に反する。`agent-telemetry.toml` の `user` キーや `[server]` セクションは agent に紐づかない属性。
3. **lifecycle と置き場所の不整合**: `agent-telemetry push` は cron / launchd / systemd timer から起動する独立プロセス（[0028](closed/0028-feat-server-push-client.md) で確定）であり、Claude Code の起動とは無関係。Claude Code 設定ディレクトリにあると、ユーザが「Claude Code を停止すれば push も止まる」と誤解しうる。
4. **誤解によるサポート流出のリスク**: 「Claude Code の設定」と認識したユーザが Anthropic に問い合わせる経路ができてしまう。具体名が `agent-telemetry.toml` である以上、ファイル名衝突の実害は小さいが、ディレクトリの所有者が誰かは UI / CLI の信号として効く。

### 検討した代替

| 案 | 採否 | 理由 |
|---|---|---|
| `~/.config/agent-telemetry/config.toml`（XDG） | **採用** | `os.UserConfigDir()` でクロスプラットフォーム対応、agent-agnostic、責務の所在が明確 |
| `~/.agent-telemetry/config.toml`（dotfile 直下） | 不採用 | XDG が標準として浸透している現在、dotfile 散らかしは古いパターン |
| `~/.claude/agent-telemetry.toml` を維持 | 不採用 | 上述の問題が残る |
| `~/.codex/agent-telemetry.toml` に移す | 不採用 | Claude 寄りから Codex 寄りに変えるだけで agent-agnostic にならない |
| filename を `server.toml` にする | 不採用 | `[user]` キーや将来の `[client]` 等を含む前提で「server 用設定」という限定は誤解を招く。`config.toml` のほうが汎用的 |

## 対応方針

### 解決パス

1. **新パス**: `~/.config/agent-telemetry/config.toml`
   - `XDG_CONFIG_HOME` が設定されていれば `$XDG_CONFIG_HOME/agent-telemetry/config.toml`
   - Go 標準の `os.UserConfigDir()` を使えば darwin / linux / windows の差異も自然に吸収される（macOS では `~/Library/Application Support/` を返すが、本ツールは XDG 準拠を明示するため linux / macOS 共通で `~/.config/` に固定する判断もありうる。実装時に確定）
2. **旧パス fallback**: `~/.claude/agent-telemetry.toml` を読み続ける（新パスが存在しない場合のみ）。読み込み時に stderr に migration warning を 1 行出す
3. **migration warning の dedup**: 同一プロセス内では 1 回のみ出す（push は cron / timer から起動するが、warning は呼び出しごとに出る — それは想定内。問題は 1 プロセス内で `userid.Resolve()` と `serverclient.LoadConfig()` の両方が読みに行くケースで 2 回出ること）
4. **doctor / setup**: `agent-telemetry doctor` で migration 状態を診断（旧パスを読んでいる場合は推奨アクションを表示）、`agent-telemetry setup` でも案内を出す
5. **docs 更新**: `docs/spec.md` / `docs/design.md` / `docs/setup.md` / `docs/usage.md` / `site/content/explain/` 配下の該当箇所を新パスに更新。docs/spec.md には旧パス fallback の挙動を明記する

### 受け入れ条件

- [ ] `internal/serverclient/config.go` の `ConfigPath()` / `LoadConfig()` が新パス優先・旧パス fallback で動く
- [ ] `internal/userid/userid.go` の `ConfigPath()` / `Resolve()` が同様に動く
- [ ] `XDG_CONFIG_HOME` の override が効く（環境変数を指定すればその配下を読む）
- [ ] 新パス・旧パスのどちらも存在しない場合は warning を出さず（または dedup された 1 回のみ）、exit code 0 で正常終了する。cron 安全性を維持する
- [ ] migration warning は同一プロセス内で 1 回のみ stderr に出る（`sync.Once` 等で dedup）
- [ ] テストで以下を固定する:
  - 新パスのみ存在 → 新パスを読む、warning なし
  - 旧パスのみ存在 → 旧パスを読む、warning 1 回
  - 両方存在 → 新パスを読む、warning なし
  - どちらも存在しない → no-op、warning なし
  - `XDG_CONFIG_HOME` 上書き → その配下を読む
- [ ] `cmd/agent-telemetry/main.go:236` のエラーメッセージが新パスに更新される
- [ ] `agent-telemetry doctor` が migration 状態を表示する
- [ ] `agent-telemetry setup` の出力例が新パスに更新される
- [ ] `docs/spec.md` / `docs/design.md` / `docs/setup.md` / `docs/usage.md` および `site/content/explain/` の該当箇所が新パスに更新される（旧パス fallback の挙動も明記）
- [ ] 旧パス読み込みの deprecation cycle（いつ削除するか）は本 issue では決めず、別途決定する。docs に「将来的に削除予定」の旨だけ記載する
