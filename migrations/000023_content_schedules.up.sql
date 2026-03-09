-- Content schedules: recurring social posting rules
CREATE TABLE content_schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    owner_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    content_source VARCHAR(20) NOT NULL DEFAULT 'agent',
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    prompt TEXT,
    cron_expression VARCHAR(100) NOT NULL,
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    cron_job_id VARCHAR(36),
    last_run_at TIMESTAMPTZ,
    last_status VARCHAR(20),
    last_error TEXT,
    posts_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_content_schedules_owner ON content_schedules(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_content_schedules_cron_job ON content_schedules(cron_job_id) WHERE cron_job_id IS NOT NULL;

CREATE TABLE content_schedule_pages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    schedule_id UUID NOT NULL REFERENCES content_schedules(id) ON DELETE CASCADE,
    page_id UUID NOT NULL REFERENCES social_pages(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(schedule_id, page_id)
);
CREATE INDEX idx_content_schedule_pages_schedule ON content_schedule_pages(schedule_id);

CREATE TABLE content_schedule_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    schedule_id UUID NOT NULL REFERENCES content_schedules(id) ON DELETE CASCADE,
    post_id UUID REFERENCES social_posts(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL,
    error TEXT,
    content_preview TEXT,
    pages_targeted INT NOT NULL DEFAULT 0,
    pages_published INT NOT NULL DEFAULT 0,
    duration_ms BIGINT,
    ran_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_content_schedule_logs_schedule ON content_schedule_logs(schedule_id);
CREATE INDEX idx_content_schedule_logs_ran ON content_schedule_logs(ran_at);
