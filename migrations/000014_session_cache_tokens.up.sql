ALTER TABLE project_sessions
  ADD COLUMN IF NOT EXISTS cache_read_tokens bigint NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cache_creation_tokens bigint NOT NULL DEFAULT 0;

-- Backfill from existing result logs
WITH result_events AS (
  SELECT
    session_id,
    SUM(COALESCE((content->'usage'->>'cache_read_input_tokens')::bigint, 0)) AS total_cache_read,
    SUM(COALESCE((content->'usage'->>'cache_creation_input_tokens')::bigint, 0)) AS total_cache_creation
  FROM project_session_logs
  WHERE event_type = 'result'
  GROUP BY session_id
)
UPDATE project_sessions s
SET
  cache_read_tokens = r.total_cache_read,
  cache_creation_tokens = r.total_cache_creation
FROM result_events r
WHERE s.id = r.session_id;
