ALTER TABLE project_sessions
  DROP COLUMN IF EXISTS cache_read_tokens,
  DROP COLUMN IF EXISTS cache_creation_tokens;
