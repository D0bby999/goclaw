DROP INDEX IF EXISTS idx_news_sources_category;
ALTER TABLE news_sources DROP COLUMN IF EXISTS category;
