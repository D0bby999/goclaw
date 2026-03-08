CREATE TABLE social_pages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    account_id UUID NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
    page_id VARCHAR(255) NOT NULL,
    page_name VARCHAR(255),
    page_token TEXT,
    page_type VARCHAR(50) DEFAULT 'page',
    avatar_url TEXT,
    is_default BOOLEAN DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) DEFAULT 'active',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(account_id, page_id)
);

CREATE INDEX idx_social_pages_account ON social_pages(account_id);
