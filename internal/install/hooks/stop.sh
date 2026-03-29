#!/bin/bash
# Claude Code Hook: Stop でセッション終了時に backfill + sync-db を実行する
# backfill: PR URL 補完・マージ判定（cursor で増分処理）
# sync-db:  JSONL/log → SQLite 変換

hitl-metrics backfill && hitl-metrics sync-db
