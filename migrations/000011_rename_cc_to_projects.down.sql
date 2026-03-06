-- Revert Projects back to CC tables
ALTER TABLE projects RENAME TO cc_projects;
ALTER TABLE project_sessions RENAME TO cc_sessions;
ALTER TABLE project_session_logs RENAME TO cc_session_logs;

-- Revert indexes
ALTER INDEX idx_projects_owner RENAME TO idx_cc_projects_owner;
ALTER INDEX idx_projects_team RENAME TO idx_cc_projects_team;
ALTER INDEX idx_project_sessions_project RENAME TO idx_cc_sessions_project;
ALTER INDEX idx_project_sessions_status RENAME TO idx_cc_sessions_status;
ALTER INDEX idx_project_session_logs_session RENAME TO idx_cc_session_logs_session;
