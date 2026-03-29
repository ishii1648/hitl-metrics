# ADR-020: transcript_stats にトークン使用量カラムを追加して API コスト試算を容易にする

## ステータス

Draft（凍結） — HITL 計測への直接的貢献が実証されるまで実装を保留。コスト試算が必要な場合はスタンドアロンスクリプトで対応する。

## 関連 ADR

- 関連: ADR-018（transcript_stats を参照する pr_metrics VIEW と同じ領域）

## コンテキスト

直近1ヶ月の Claude Code 利用量から API 課金モードへの移行コストを試算しようとした際、以下の問題があった。

- `transcript_stats` テーブルにはトークン使用量（input / output / cache）が記録されていない
- `session-meta` JSON には `input_tokens` / `output_tokens` はあるが、`cache_creation_input_tokens` / `cache_read_input_tokens` が欠落しており API コスト計算に使えない
- コスト試算のためには 130 セッション分の transcript JSONL を個別に読んで usage フィールドを集計する必要があり、非常に手間がかかった

実際の API コストはキャッシュトークンが支配的（cache_write + cache_read ≈ コスト全体の 90%）であり、これを DB に記録しなければ SQL クエリ1つでコスト試算することができない。

## 設計案

### 案A: transcript_stats テーブルにカラム追加（採用）

`sync-db` 時に transcript JSONL の各 `assistant` メッセージの `usage` フィールドを集計し、セッション単位の合計を `transcript_stats` に保存する。

追加カラム:

| カラム名 | 型 | 意味 |
|---|---|---|
| `input_tokens` | INTEGER | API input_tokens の合計（キャッシュ除く） |
| `output_tokens` | INTEGER | API output_tokens の合計 |
| `cache_write_tokens` | INTEGER | cache_creation_input_tokens の合計 |
| `cache_read_tokens` | INTEGER | cache_read_input_tokens の合計 |
| `model` | TEXT | セッションで使用されたモデル（複数の場合は最後のモデル） |

これにより以下の SQL でコスト試算が可能になる:

```sql
SELECT
    model,
    SUM(input_tokens)       AS total_input,
    SUM(output_tokens)      AS total_output,
    SUM(cache_write_tokens) AS total_cache_write,
    SUM(cache_read_tokens)  AS total_cache_read
FROM transcript_stats
JOIN sessions USING (session_id)
WHERE sessions.timestamp >= '2026-02-28'
  AND sessions.is_subagent = 0
GROUP BY model;
```

### 案B: 別テーブル（token_stats）として分離（却下）

transcript_stats はセッション単位の集計統計テーブルであり、トークン使用量も同じ粒度で管理するのが自然。別テーブルにすると JOIN が増えるだけでメリットがない。

### 変更が必要なファイル（affected-scope）

| ファイル / パッケージ | 変更内容 |
|---|---|
| `internal/syncdb/schema.go` | transcript_stats テーブルに 5 カラム追加 |
| `internal/syncdb/syncdb.go` | transcript 集計ロジックで usage フィールドを読み取り合計をセット |
| `internal/syncdb/syncdb_test.go` | 新カラムの検証テスト追加 |
