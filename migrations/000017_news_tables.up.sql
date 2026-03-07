-- News sources: configured scraping targets
CREATE TABLE news_sources (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    source_type     VARCHAR(50)  NOT NULL,
    config          JSONB        NOT NULL DEFAULT '{}',
    enabled         BOOLEAN      NOT NULL DEFAULT true,
    scrape_interval VARCHAR(50)  DEFAULT 'daily',
    last_scraped_at TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  DEFAULT NOW()
);

CREATE INDEX idx_news_sources_agent ON news_sources(agent_id);
CREATE INDEX idx_news_sources_type  ON news_sources(source_type);

-- News items: scraped + AI-analyzed articles
CREATE TABLE news_items (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    source_id    UUID REFERENCES news_sources(id) ON DELETE SET NULL,
    agent_id     UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    url_hash     VARCHAR(64)  NOT NULL,
    url          TEXT         NOT NULL,
    title        TEXT         NOT NULL,
    content      TEXT,
    summary      TEXT,
    categories   TEXT[]       DEFAULT '{}',
    tags         TEXT[]       DEFAULT '{}',
    sentiment    VARCHAR(20),
    insights     JSONB        DEFAULT '{}',
    source_type  VARCHAR(50),
    source_name  VARCHAR(255),
    published_at TIMESTAMPTZ,
    scraped_at   TIMESTAMPTZ  DEFAULT NOW(),
    created_at   TIMESTAMPTZ  DEFAULT NOW(),

    CONSTRAINT uq_news_items_url_hash UNIQUE (agent_id, url_hash)
);

CREATE INDEX idx_news_items_agent      ON news_items(agent_id);
CREATE INDEX idx_news_items_scraped    ON news_items(scraped_at DESC);
CREATE INDEX idx_news_items_categories ON news_items USING GIN(categories);
CREATE INDEX idx_news_items_source     ON news_items(source_id);
