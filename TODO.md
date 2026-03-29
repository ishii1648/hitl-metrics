# TODO

hitl-metrics の開発タスクを管理する。完了したら CHANGELOG.md に記録して削除する。

## 実装待ち

- GitHub Release でバイナリを自動ビルド・配布
  - GitHub Actions でタグ push 時に goreleaser でマルチプラットフォームバイナリを生成
  - `hitl-metrics install` の前提をバイナリダウンロードに変更し、ユーザの手元ビルドを不要にする

## 検討中

- Bash コマンドのコンテキスト消費監視
  - `PostToolUse` Hook（`posttooluse-track.sh`）で Bash コマンドの stdout サイズを記録
  - redirect-to-tools をすり抜けた正当な Bash コマンドのうち、出力が大きいものを特定
  - 定期集計で「常連犯」コマンドを可視化し、対策要否を判断する
- retro-pr との連携
  - PR の下位・上位10%ずつは自動で retro-pr 実行
  - 結果を PR と関連付けて表示

## 進行中

- GitHub Release でバイナリを自動ビルド・配布
  - [ ] GitHub Actions でタグ push 時に goreleaser でマルチプラットフォームバイナリを生成
  - [ ] `hitl-metrics install` でバイナリに embedded された hook スクリプトを展開・登録
  - [ ] docs/setup.md を Go ビルド不要のバイナリダウンロード手順に変更
