---
name: auto-fix
description: PR push 後の GitHub Actions CI を背景待機し、全 check 完了後に失敗ジョブのログを一括取得して原因診断 → まとめて修正 → 再 push のループを自動実行する。`git-ship` 完了直後に自動連結されるほか、「ci 直して」「fix ci error」「auto-fix」「PR の CI 見て」「CI 監視して」などのトリガーで起動。
allowed-tools: Bash, Read, Edit, Write, Skill
version: 0.2.0
---

# auto-fix

PR を push した後の CI を見守り、失敗があれば原因を特定して修正・再 push を繰り返す。すべて pass するか、ユーザー介入が必要な失敗に達するまでループする。

## 中核ツール: Bash run_in_background

**この skill は `Bash` の `run_in_background: true` で「全 check が確定するまで待つ until ループ」を 1 個 background 起動する。** Monitor tool は使わない（per-event 通知は CI fix の文脈では token と inference を浪費するため）。

理由:

- CI 失敗は連鎖して関連していることが多い（go.mod bump 1 つで lint / govulncheck / build が同時に落ちる類）。**まとめて 1 commit で修正する** ほうがレビュー単位として綺麗。per-event 通知は逐次修正に誘導してしまう
- 多くのプロジェクトで build / e2e が支配的時間を占めるので、他の check が早期完了しても並行で出来ることは少ない
- Monitor tool 公式 description にも「**One**（"tell me when the build finishes"）→ use **Bash with `run_in_background`**」と明記されている

`gh run watch` / `gh pr checks --watch` の **同期** blocking は禁止（Bash の 10 分 timeout を超える CI で死ぬし、その間 Claude が他作業に進めない）。あくまで **`run_in_background: true` の until ループ** にする。

## 前提条件

実行前に以下を確認する。満たさなければ実行しない：

1. `feat/`, `fix/`, `docs/`, `chore/` のいずれかで始まるブランチ上にいる（`main` 上では実行しない）
2. そのブランチ向けに open PR が存在する（`gh pr view --json number` で取得可能）
3. `gh` CLI が認証済みで、対象 repo にアクセスできる
4. working tree がクリーン（push 済みで未コミット変更がない）

## コアフロー

### Step 1: PR 番号を特定

```bash
gh pr view --json number -q '.number'
```

取得した番号を以降 `<PR>` として扱う。

### Step 2: Bash run_in_background で待機

`Bash` tool に **`run_in_background: true`** を付けて以下を起動する。`<PR>` は実数で置き換える:

```bash
until s=$(gh pr checks <PR> --json name,bucket 2>/dev/null) && \
      jq -e 'length>0 and all(.bucket!="pending")' <<<"$s" >/dev/null; do
  sleep 30
done
echo "ALL_DONE"
gh pr checks <PR>
```

- `gh pr checks --json` が一度でも失敗（network 5xx 等）したら次の cycle で retry される（`until` の condition で fail を許容）
- 全 check が `pending` を脱した瞬間に loop が break、最終 snapshot を表示して終了
- 完了通知は **1 回だけ** 会話に届く

`run_in_background: true` を渡したあと、Claude は通知を待つ（poll しない、自前で sleep しない）。background 中に並行で別作業（ローカル再現・ログ事前読み・前回 PR の確認など）をしても良い。

### Step 3: 結果判定

完了通知が届いたら最終 snapshot を確認する:

```bash
gh pr checks <PR>
```

- 全 check が `pass` → **終了**。PR URL を出して報告して完了。
- `fail` がある → Step 4 へ。
- `cancelled` / `skipping` のみ → 状況を見て手動再実行か終了を判断。

### Step 4: 失敗ログ取得と原因診断

**失敗 check は全部まとめて並列取得する。** 各 URL から `<run-id>` と `<job-id>` を抽出し（URL 形式: `.../actions/runs/<run-id>/job/<job-id>`）:

```bash
gh run view <run-id> --job <job-id> --log-failed | tail -80
```

を **1 メッセージ内の複数 Bash 呼び出し** で並列実行する（依存関係がないため）。

#### よくある失敗パターン
- **golangci-lint バージョン不整合**: `go.mod` の go directive を上げると古い golangci-lint が target できなくなる → action の version を上げ、`.golangci.yml` を新 syntax に migrate
- **govulncheck stdlib vuln**: 検出された vuln の `Fixed in:` バージョンを見て `go.mod` の go directive を bump
- **Docker build go version mismatch**: `Dockerfile` の golang base image を `go.mod` の directive と合わせる
- **gofmt drift**: `gofmt -w` で再整形
- **go mod tidy drift**: `go mod tidy` で go.mod / go.sum を更新
- **transient runner failure（network / 5xx）**: `gh run rerun <run-id> --failed` で再実行のみ

### Step 5: まとめて修正 → 1 commit → push

1. 失敗ログを総合し、**根本原因が共通するものは 1 つの修正にまとめる**（例: go.mod bump 1 つで lint / govulncheck / build が一括解決するなら 1 commit）
2. ローカルで可能な限り再現して緑を確認（`go vet ./... && go build ./... && go test -race ./...` 等）
3. 関連ファイルだけ stage（`git add -A` は使わない）
4. `contextual-commit` skill を呼んで commit を作成
5. `git push`

### Step 6: Step 2 に戻る

新しい push で CI が再走するので、再度 Step 2 の until ループを `run_in_background` で arm する。

## ループ終了条件

- **成功**: 全 check pass → PR URL を出して終了
- **試行回数上限**: 同一 PR に対する自動 fix push が **5 回** に達した → ユーザーに報告して停止
- **同じエラーの繰り返し**: 直前の修正と同じ症状が連続 2 回出た → 自動修正不能と判断、ユーザーに報告して停止
- **手動介入必須の失敗**: secrets 不足、外部サービス障害、権限エラー、`required reviewers` 未充足など → 状況を報告して停止
- **ローカル再現で破綻**: ローカル環境で修正が再現できない（CI 固有の環境差） → 報告して停止

## ガードレール

- **destructive action 禁止**: `git push --force`, `git reset --hard`, branch 削除, PR close は行わない
- **`--no-verify` 禁止**: pre-commit hook が落ちたら根本原因を調べる
- **推測による fix 禁止**: ログから根本原因が読み取れない場合は修正せずユーザーに報告
- **scope crawl 禁止**: CI 緑化に必要最小限の変更のみ。「ついでに refactor」は別 PR
- **secrets を log に出さない**: ログ取得時は `tail` で末尾だけに絞り、誤って secrets を session に流さない
- **同期 watch 禁止**: `gh run watch` / `gh pr checks --watch` のような blocking 同期 watch は使わない（Bash 10 分 timeout で死ぬ）

## 報告フォーマット

完了時は以下のいずれかで報告する：

- 成功: `CI 全 check 緑。PR: <URL>`
- 試行上限: `自動 fix を 5 回試したが緑にならず停止。最後の失敗: <job> / <要約>`
- 介入必須: `手動対応が必要な失敗を検知して停止。<job>: <理由>`

## 例

git-ship が `https://github.com/owner/repo/pull/57` を返した直後:

1. PR 番号 57 を取得
2. `Bash run_in_background` で until ループを arm（"待つだけ" の background task）
3. 完了通知 1 回 → `gh pr checks 57` で `lint: fail`, `govulncheck: fail`, `build: fail` を一覧
4. 3 件の `--log-failed` を **並列に** 取得し診断:
   - lint: golangci-lint v1.64 が Go 1.25 target 不可
   - govulncheck: stdlib に GO-2026-XXXX
   - build: Dockerfile が古い base image
5. **3 件まとめて 1 commit** で修正（go.mod bump + golangci-lint v2 移行 + Dockerfile bump）
6. push
7. Step 2 に戻る → 全 pass 通知 → 終了
