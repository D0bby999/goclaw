ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS max_duration int NOT NULL DEFAULT 0;

COMMENT ON COLUMN projects.max_duration IS 'Max session duration in seconds. 0 = unlimited.';
