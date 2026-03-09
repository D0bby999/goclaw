ALTER TABLE news_sources ADD COLUMN IF NOT EXISTS category VARCHAR(100);
CREATE INDEX idx_news_sources_category ON news_sources(category) WHERE category IS NOT NULL;
