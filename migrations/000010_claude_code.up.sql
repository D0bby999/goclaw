-- Claude Code orchestration tables (managed mode only)

CREATE TABLE cc_projects (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,
    work_dir        TEXT NOT NULL,
    description     TEXT DEFAULT '',
    allowed_tools   JSONB DEFAULT '[]',
    env_vars        BYTEA,
    claude_config   JSONB DEFAULT '{}',
    max_sessions    INT NOT NULL DEFAULT 3,
    owner_id        VARCHAR(255) NOT NULL,
    team_id         UUID REFERENCES teams(id) ON DELETE SET NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cc_projects_owner ON cc_projects(owner_id) WHERE status = 'active';
CREATE INDEX idx_cc_projects_team ON cc_projects(team_id) WHERE team_id IS NOT NULL AND status = 'active';

CREATE TABLE cc_sessions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    project_id        UUID NOT NULL REFERENCES cc_projects(id) ON DELETE CASCADE,
    claude_session_id TEXT,
    label             VARCHAR(500) DEFAULT '',
    status            VARCHAR(20) NOT NULL DEFAULT 'starting',
    pid               INT,
    started_by        VARCHAR(255) NOT NULL,
    input_tokens      BIGINT DEFAULT 0,
    output_tokens     BIGINT DEFAULT 0,
    cost_usd          NUMERIC(12,6) DEFAULT 0,
    error             TEXT,
    started_at        TIMESTAMPTZ DEFAULT NOW(),
    stopped_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ DEFAULT NOW(),
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cc_sessions_project ON cc_sessions(project_id, created_at DESC);
CREATE INDEX idx_cc_sessions_status ON cc_sessions(status) WHERE status IN ('starting', 'running');

CREATE TABLE cc_session_logs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    session_id  UUID NOT NULL REFERENCES cc_sessions(id) ON DELETE CASCADE,
    event_type  VARCHAR(50) NOT NULL,
    content     JSONB NOT NULL,
    seq         INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_cc_session_logs_session ON cc_session_logs(session_id, seq);
