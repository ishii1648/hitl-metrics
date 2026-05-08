# GoReleaser tag push 動作確認 + リリース

Created: 2026-05-08
Model: Opus 4.7

## 概要

リポジトリリネーム（hitl-metrics → agent-telemetry）の最終フェーズとして、tag push 経由で GoReleaser が `agent-telemetry_<os>_<arch>.tar.gz` 形式のバイナリを生成することを確認する。

## 根拠

リネームのフェーズ 3〜5 は完了済みだが、最終フェーズ（タグ push 動作確認）が未実施。GoReleaser 設定が agent-telemetry 名で正しく動作するかは tag push しないと分からない。リネーム決定の背景・決定事項（D1〜D4）と BREAKING CHANGE 一覧は `docs/history.md` 「8. リポジトリ名変更 — hitl-metrics → agent-telemetry（2026-05-04）」を参照。

## 対応方針

- 任意の patch tag を push（例: `v0.x.y`）
- GoReleaser の GitHub Actions ワークフローが完走することを確認
- 生成された artifact 名が `agent-telemetry_<os>_<arch>.tar.gz` であることを確認
- リリースノートが正しく生成されることを確認

## 受け入れ条件

- [ ] tag push で GoReleaser が完走
- [ ] artifact 名が `agent-telemetry` プレフィックスで生成される
- [ ] GitHub Releases ページに正しいリリースが表示される
