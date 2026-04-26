package syncdb

const createTablesSQL = `
DROP TABLE IF EXISTS permission_events;
DROP TABLE IF EXISTS transcript_stats;
DROP TABLE IF EXISTS sessions;
DROP VIEW IF EXISTS pr_metrics;

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

CREATE TABLE permission_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp  TEXT NOT NULL,
    session_id TEXT NOT NULL,
    tool       TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE transcript_stats (
    session_id        TEXT PRIMARY KEY,
    tool_use_total    INTEGER NOT NULL DEFAULT 0,
    mid_session_msgs  INTEGER NOT NULL DEFAULT 0,
    ask_user_question INTEGER NOT NULL DEFAULT 0,
    is_ghost          INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_sessions_pr_url ON sessions(pr_url);
CREATE INDEX idx_sessions_repo ON sessions(repo);
CREATE INDEX idx_perm_session ON permission_events(session_id);
CREATE INDEX idx_perm_ts ON permission_events(timestamp);

CREATE VIEW pr_metrics AS
SELECT
    s.pr_url,
    MAX(s.task_type) AS task_type,
    COUNT(DISTINCT s.session_id) AS session_count,
    COALESCE(SUM(ts.tool_use_total), 0) AS tool_use_total,
    COALESCE(SUM(ts.mid_session_msgs), 0) AS mid_session_msgs,
    COALESCE(SUM(ts.ask_user_question), 0) AS ask_user_question,
    COALESCE(SUM(pe_agg.perm_count), 0) AS perm_count,
    MAX(s.review_comments) AS review_comments,
    MAX(s.changes_requested) AS changes_requested,
    CASE WHEN COALESCE(SUM(ts.tool_use_total), 0) > 0
         THEN ROUND(COALESCE(SUM(pe_agg.perm_count), 0) * 100.0 / SUM(ts.tool_use_total), 1)
         ELSE NULL END AS perm_rate
FROM sessions s
LEFT JOIN transcript_stats ts ON s.session_id = ts.session_id
LEFT JOIN (
    SELECT session_id, COUNT(*) AS perm_count
    FROM permission_events
    GROUP BY session_id
) pe_agg ON s.session_id = pe_agg.session_id
WHERE s.pr_url != ''
  AND s.is_subagent = 0
  AND s.is_merged = 1
  AND COALESCE(ts.is_ghost, 0) = 0
  AND s.repo NOT IN ('ishii1648/dotfiles')
GROUP BY s.pr_url;
`
