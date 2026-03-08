-- Social accounts: connected social platform accounts with encrypted OAuth tokens
CREATE TABLE social_accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    owner_id VARCHAR(255) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    platform_user_id VARCHAR(255) NOT NULL,
    platform_username VARCHAR(255),
    display_name VARCHAR(255),
    avatar_url TEXT,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    scopes TEXT[],
    metadata JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    connected_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_social_accounts_unique_platform_user ON social_accounts(platform, platform_user_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_social_accounts_owner ON social_accounts(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_social_accounts_platform ON social_accounts(platform) WHERE deleted_at IS NULL;

-- Social posts: post content with scheduling and status tracking
CREATE TABLE social_posts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    owner_id VARCHAR(255) NOT NULL,
    title VARCHAR(500),
    content TEXT NOT NULL,
    post_type VARCHAR(20) NOT NULL DEFAULT 'post',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    scheduled_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    post_group_id UUID,
    parent_post_id UUID REFERENCES social_posts(id) ON DELETE SET NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_social_posts_owner ON social_posts(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_social_posts_status ON social_posts(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_social_posts_scheduled ON social_posts(scheduled_at) WHERE status = 'scheduled' AND deleted_at IS NULL;
CREATE INDEX idx_social_posts_group ON social_posts(post_group_id) WHERE post_group_id IS NOT NULL AND deleted_at IS NULL;

-- Social post targets: per-account publish targets (many-to-many)
CREATE TABLE social_post_targets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    post_id UUID NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
    account_id UUID NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
    platform_post_id VARCHAR(255),
    platform_url TEXT,
    adapted_content TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error TEXT,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(post_id, account_id)
);
CREATE INDEX idx_social_post_targets_post ON social_post_targets(post_id);
CREATE INDEX idx_social_post_targets_account ON social_post_targets(account_id);

-- Social post media: media attachments for posts
CREATE TABLE social_post_media (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    post_id UUID NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
    media_type VARCHAR(20) NOT NULL,
    url TEXT NOT NULL,
    thumbnail_url TEXT,
    filename VARCHAR(255),
    mime_type VARCHAR(100),
    file_size BIGINT,
    width INT,
    height INT,
    duration_seconds INT,
    sort_order INT NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_social_post_media_post ON social_post_media(post_id);

-- Social OAuth states: temporary CSRF protection for OAuth flow
CREATE TABLE social_oauth_states (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    platform VARCHAR(50) NOT NULL,
    state VARCHAR(255) NOT NULL UNIQUE,
    owner_id VARCHAR(255) NOT NULL,
    redirect_url TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_social_oauth_states_state ON social_oauth_states(state);
CREATE INDEX idx_social_oauth_states_expires ON social_oauth_states(expires_at);
