package syncdb

const createTablesSQL = `
DROP VIEW IF EXISTS pr_metrics;
DROP TABLE IF EXISTS permission_events;
DROP TABLE IF EXISTS transcript_stats;
DROP TABLE IF EXISTS sessions;

CREATE TABLE sessions (
    session_id        TEXT PRIMARY KEY,
    timestamp         TEXT NOT NULL,
    cwd               TEXT NOT NULL DEFAULT '',
    repo              TEXT NOT NULL DEFAULT '',
    branch            TEXT NOT NULL DEFAULT '',
    pr_url            TEXT NOT NULL DEFAULT '',
    transcript        TEXT NOT NULL DEFAULT '',
    parent_session_id TEXT NOT NULL DEFAULT '',
    is_subagent       INTEGER NOT NULL DEFAULT 0,
    backfill_checked  INTEGER NOT NULL DEFAULT 0,
    is_merged         INTEGER NOT NULL DEFAULT 0,
    task_type         TEXT NOT NULL DEFAULT '',
    review_comments   INTEGER NOT NULL DEFAULT 0,
    changes_requested INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE transcript_stats (
    session_id         TEXT PRIMARY KEY,
    tool_use_total     INTEGER NOT NULL DEFAULT 0,
    mid_session_msgs   INTEGER NOT NULL DEFAULT 0,
    ask_user_question  INTEGER NOT NULL DEFAULT 0,
    input_tokens       INTEGER NOT NULL DEFAULT 0,
    output_tokens      INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens  INTEGER NOT NULL DEFAULT 0,
    model              TEXT NOT NULL DEFAULT '',
    is_ghost           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_sessions_pr_url ON sessions(pr_url);
CREATE INDEX idx_sessions_repo ON sessions(repo);

CREATE VIEW pr_metrics AS
SELECT
    pm.*,
    (pm.input_tokens + pm.output_tokens + pm.cache_write_tokens + pm.cache_read_tokens) AS total_tokens,
    CASE WHEN pm.session_count > 0
         THEN ROUND((pm.input_tokens + pm.output_tokens + pm.cache_write_tokens + pm.cache_read_tokens) * 1.0 / pm.session_count, 1)
         ELSE NULL END AS tokens_per_session,
    CASE WHEN pm.tool_use_total > 0
         THEN ROUND((pm.input_tokens + pm.output_tokens + pm.cache_write_tokens + pm.cache_read_tokens) * 1.0 / pm.tool_use_total, 1)
         ELSE NULL END AS tokens_per_tool_use,
    CASE WHEN (pm.input_tokens + pm.output_tokens + pm.cache_write_tokens + pm.cache_read_tokens) > 0
         THEN ROUND(1000000.0 / (pm.input_tokens + pm.output_tokens + pm.cache_write_tokens + pm.cache_read_tokens), 2)
         ELSE NULL END AS pr_per_million_tokens
FROM (
    SELECT
        s.pr_url,
        MAX(s.task_type) AS task_type,
        MAX(ts.model) AS model,
        COUNT(DISTINCT s.session_id) AS session_count,
        COALESCE(SUM(ts.tool_use_total), 0) AS tool_use_total,
        COALESCE(SUM(ts.mid_session_msgs), 0) AS mid_session_msgs,
        COALESCE(SUM(ts.ask_user_question), 0) AS ask_user_question,
        COALESCE(SUM(ts.input_tokens), 0) AS input_tokens,
        COALESCE(SUM(ts.output_tokens), 0) AS output_tokens,
        COALESCE(SUM(ts.cache_write_tokens), 0) AS cache_write_tokens,
        COALESCE(SUM(ts.cache_read_tokens), 0) AS cache_read_tokens,
        MAX(s.review_comments) AS review_comments,
        MAX(s.changes_requested) AS changes_requested
    FROM sessions s
    LEFT JOIN transcript_stats ts ON s.session_id = ts.session_id
    WHERE s.pr_url != ''
      AND s.is_subagent = 0
      AND s.is_merged = 1
      AND COALESCE(ts.is_ghost, 0) = 0
      AND s.repo NOT IN ('ishii1648/dotfiles')
    GROUP BY s.pr_url
) pm;
`
