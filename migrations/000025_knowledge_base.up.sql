-- Knowledge Base tables for RAG (Retrieval-Augmented Generation)

-- Collections: per-agent grouping of documents
CREATE TABLE kb_collections (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name        VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    metadata    JSONB DEFAULT '{}',
    doc_count   INT NOT NULL DEFAULT 0,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_kb_col_agent_name ON kb_collections(agent_id, name);
CREATE INDEX idx_kb_col_agent ON kb_collections(agent_id);

-- Documents: individual files within a collection
CREATE TABLE kb_documents (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    collection_id   UUID NOT NULL REFERENCES kb_collections(id) ON DELETE CASCADE,
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    filename        VARCHAR(500) NOT NULL,
    mime_type       VARCHAR(100) NOT NULL DEFAULT 'text/plain',
    file_size       BIGINT NOT NULL DEFAULT 0,
    storage_key     VARCHAR(500) NOT NULL DEFAULT '',
    content_hash    VARCHAR(64) NOT NULL,
    version         INT NOT NULL DEFAULT 1,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message   TEXT,
    chunk_count     INT NOT NULL DEFAULT 0,
    embedded_count  INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kb_doc_collection ON kb_documents(collection_id);
CREATE INDEX idx_kb_doc_agent ON kb_documents(agent_id);
CREATE INDEX idx_kb_doc_status ON kb_documents(status);

-- Chunks: searchable text segments with embeddings
CREATE TABLE kb_chunks (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    document_id UUID NOT NULL REFERENCES kb_documents(id) ON DELETE CASCADE,
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL DEFAULT 0,
    text        TEXT NOT NULL,
    hash        VARCHAR(64) NOT NULL,
    start_line  INT NOT NULL DEFAULT 0,
    end_line    INT NOT NULL DEFAULT 0,
    embedding   vector(1536),
    tsv         tsvector GENERATED ALWAYS AS (to_tsvector('simple', text)) STORED,
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kb_chunk_doc ON kb_chunks(document_id);
CREATE INDEX idx_kb_chunk_agent ON kb_chunks(agent_id);
CREATE INDEX idx_kb_chunk_tsv ON kb_chunks USING GIN(tsv);
CREATE INDEX idx_kb_chunk_vec ON kb_chunks USING hnsw(embedding vector_cosine_ops);
