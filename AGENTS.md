# agent-telemetry

Codex の PR 単位のトークン消費効率を追跡・可視化する計測ツール（hook・CLI・ダッシュボード）。

## ドキュメント構成

- `docs/spec.md` — 外部契約（CLI コマンド・hook 仕様・データモデル・ダッシュボード）
- `docs/design.md` — 実装方針と設計判断
- `docs/history.md` — 過去の経緯と廃止された設計
- `docs/setup.md` / `docs/usage.md` — セットアップと運用
- `docs/archive/adr/` — 過去の意思決定記録（旧 ADR 形式、参照のみ）

新規の設計判断は ADR を作成しない。`docs/design.md` を更新し、Contextual Commits のアクション行で「なぜ」を記録する。大きな方針転換は `docs/history.md` にも追記する。

## セッションモード

### 設計セッション（main ブランチ）

- 変更対象: `docs/`, `issues/`, `AGENTS.md` のみ
- コード変更禁止（Spike を除く）
- 仕様変更は `docs/spec.md` を更新する
- 実装方針の変更は `docs/design.md` を更新する。「なぜ」が複数コミットにわたる大きな転換の場合は `docs/history.md` にも追記する
- 着手可能なタスクは `issues/<NNNN>-<cat>-<slug>.md`（受け入れ条件 `- [ ]` あり）として open し、設計判断・外部依存待ちは `issues/pending/` に置く。詳細は本ファイルの「issues について」を参照

### 実装セッション（feature ブランチ / worktree）

- 対象 issue を 1 つ実装する（`issues/<NNNN>-<cat>-<slug>.md` の受け入れ条件に従う）
- worktree 作成: `gw_add feat/<task-name>`
- 受け入れ条件を満たすまで実装 → 検証 → 修正
- 完了したら最終コミットで `git mv issues/<id>-... issues/closed/<id>-...` し、末尾に `Completed:` と `## 解決方法` を追記する
- main の `docs/` は変更しない（仕様/設計の更新は merge 後に main で実施）

## 開発規約

### 意思決定の記録方針

- 仕様の変更 → `docs/spec.md` を更新
- 実装方針の変更 → `docs/design.md` を更新
- 過去の意思決定の経緯として残す価値がある転換 → `docs/history.md` に追記
- 1 コミット内で完結する判断 → Contextual Commits のアクション行で記録
- chore / リファクタなど意思決定を伴わない変更 → アクション行不要

### コミット

Contextual Commits を使用。Conventional Commits プレフィックス + 構造化されたアクション行でコミットの意図を記録する。

### ブランチ命名

`feat/`, `fix/`, `docs/`, `chore/` + kebab-case（例: `feat/add-sync-db`）

### issues について

`issues/` は **タスク兼意思決定記録** の primary store。バグ・課題・設計判断はここに Markdown としてライフサイクル管理する。GitHub Issues は使わない（ローカルで完結し、`git log` と同じ粒度で追跡できるため）。

各 issue は frontmatter で「どの層の決定か（spec / design / implementation / process）」「どの code path に effect を持つか（`affected_paths`）」を構造化して持つ。これにより、コードを編集する場所から関連する過去の意思決定を `make intent PATH=<p>` / `scripts/intent-lookup <p>` で逆引きできる。「複数コミット or 後続が参照しそうな決定」は issue 化する。1 コミットで完結する判断は Contextual Commits の action 行で十分。

#### ディレクトリ構成

```
issues/
  SEQUENCE                     # 次に発番する番号（整数 1 行）
  NNNN-<category>-<slug>.md    # open な issue
  closed/
    NNNN-<category>-<slug>.md  # 解決済み
  pending/
    NNNN-<category>-<slug>.md  # 設計判断保留・外部依存待ち
```

`issues/closed/` と `issues/pending/` には `.gitkeep` を置き、空でも git に残す。

#### 命名規則

`{seqnum}-{category}-{short-description}.md`

- `seqnum` は 4 桁ゼロパディング（`0001`, `0042`）。9999 を超えたら 5 桁に拡張する
- `seqnum` は `issues/SEQUENCE` の値を使う。issue を新規作成したら同コミットで `+1` する
- `category` は `bug` / `feat` / `doc` / `chore` / `design` のいずれか。`design` は pending に置かれることが多い
- `short-description` は kebab-case の英数字。例: `0001-bug-pr-session-misattribution.md`

#### issue ファイルの構造

```markdown
---
decision_type: spec | design | implementation | process
affected_paths:
  - internal/agent/codex/
  - cmd/agent-telemetry/setup.go
supersedes: [0023]
tags: [hooks, multi-agent, packaging]
closed_at: YYYY-MM-DD
---

# <タイトル>

Created: YYYY-MM-DD

## 概要

## 根拠

## 問題

## 対応方針
```

具体的なコード変更は PR 側に書く。ここに書くのは「なぜ対応する必要があるか」と「どの方針で進めるか」までに留める。

##### frontmatter フィールド

| フィールド | 型 | セマンティクス |
|---|---|---|
| `decision_type` | enum | `spec`（外部契約）/ `design`（内部設計）/ `implementation`（実装 detail）/ `process`（開発プロセス）。意思決定の層 |
| `affected_paths` | string[] | path snapshot。issue が effect を持つ code / docs path（`/` 終端でディレクトリを表す）。rename は `git log --follow` で逆引き側が吸収するため path 自体は更新しない |
| `supersedes` | int[] | 過去 issue 番号の配列（例: `[0023]`）。supersededBy への双方向参照は索引側で生成 |
| `tags` | string[] | free-form。当面は明示的な語彙統制を置かず、出現頻度から事後に整理する |
| `closed_at` | date | close 時に確定（`YYYY-MM-DD`）。open / pending では省略可 |

`affected_paths` は「`make intent PATH=<p>` で逆引きしたときに、この issue がヒットして欲しい path」を書く。粒度はファイル単位でもディレクトリ単位でもよい（broad な決定は親ディレクトリを、ピンポイントな決定は具体的なファイルを）。

#### ライフサイクル

| 状態 | 場所 | 移動契機 | 同時に書く内容 |
|---|---|---|---|
| open | `issues/<id>-<cat>-<slug>.md` | 検出時に新規作成 | Created / 概要 / 根拠 / 問題 / 対応方針 |
| closed | `issues/closed/<id>-<cat>-<slug>.md` | 修正 PR の最終コミットで `git mv` | 末尾に `Completed: YYYY-MM-DD` と `## 解決方法` を追記 |
| pending | `issues/pending/<id>-<cat>-<slug>.md` | 設計判断・外部依存待ちで `git mv` | 末尾に `## Pending YYYY-MM-DD` と保留理由を追記 |
| reopen | `git mv issues/closed/<id>-... issues/<id>-...` | close 後に再発見 | 末尾に `## Reopen YYYY-MM-DD` と再発の経緯を追記。`## 解決方法` は経緯として残す |

#### コミットルール

- 番号が小さい open issue から順に対応する
- issue を新規作成したら同コミットで `issues/SEQUENCE` も `+1` する（失念防止）
- 1 issue の close ごとに 1 コミットを基本とする
- 関連 PR の説明冒頭に `issues/<id>-...` へのリンクを貼る（参考: `docs/history.md` 9 番のリンク作法）

#### バグ発見時のフロー

1. `issues/SEQUENCE` を読み次の番号を決める
2. `issues/<NNNN>-bug-<slug>.md` に再現手順・原因仮説・影響範囲を書く
3. `issues/SEQUENCE` を `+1` する
4. 1 コミットでまとめる（実装着手前に記録するためのコミット）
5. 別ブランチで修正 → 修正 PR の最終コミットで `git mv issues/<id>-... issues/closed/<id>-...` し、`Completed:` と `## 解決方法` を追記する

### テスト

```fish
go test ./...                          # 全テスト
make grafana-screenshot                # E2E: Grafana スクリーンショット検証
```

### ダッシュボード変更時の必須作業

- `grafana/dashboards/agent-telemetry.json` の表示を変更した場合は、必ず `make grafana-screenshot` を実行して README 用スクリーンショット（`docs/images/dashboard-*.png`）も同じ変更に合わせて更新する（`grafana-screenshot` は `grafana-up-e2e` 経由で fixture データを使うので、画像が決定的に再現される）。
- スクリーンショット生成でポート競合が起きる場合は `GRAFANA_PORT=<unused-port> make grafana-screenshot` を使う。
- 実データで動作確認したい場合は `make grafana-up`（`~/.claude/agent-telemetry.db` を mount）。E2E と同じコンテナを使うので、切替時は片方が再作成される。
