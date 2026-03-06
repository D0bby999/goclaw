-- Rename CC tables to Projects
ALTER TABLE cc_projects RENAME TO projects;
ALTER TABLE cc_sessions RENAME TO project_sessions;
ALTER TABLE cc_session_logs RENAME TO project_session_logs;

-- Rename indexes
ALTER INDEX idx_cc_projects_owner RENAME TO idx_projects_owner;
ALTER INDEX idx_cc_projects_team RENAME TO idx_projects_team;
ALTER INDEX idx_cc_sessions_project RENAME TO idx_project_sessions_project;
ALTER INDEX idx_cc_sessions_status RENAME TO idx_project_sessions_status;
ALTER INDEX idx_cc_session_logs_session RENAME TO idx_project_session_logs_session;
