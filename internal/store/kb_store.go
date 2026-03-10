package store

import "context"

// KBCollection represents a knowledge base collection.
type KBCollection struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	DocCount    int            `json:"doc_count"`
	Status      string         `json:"status"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
}

// KBDocument represents a document in a knowledge base.
type KBDocument struct {
	ID            string `json:"id"`
	CollectionID  string `json:"collection_id"`
	AgentID       string `json:"agent_id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	FileSize      int64  `json:"file_size"`
	StorageKey    string `json:"storage_key,omitempty"`
	ContentHash   string `json:"content_hash"`
	Version       int    `json:"version"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"error_message,omitempty"`
	ChunkCount    int    `json:"chunk_count"`
	EmbeddedCount int    `json:"embedded_count"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// KBChunk represents a text chunk from a document.
type KBChunk struct {
	ID           string `json:"id"`
	DocumentID   string `json:"document_id"`
	ChunkIndex   int    `json:"chunk_index"`
	Text         string `json:"text"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	HasEmbedding bool   `json:"has_embedding"`
}

// KBSearchResult is a single result from KB search.
type KBSearchResult struct {
	DocumentID   string  `json:"document_id"`
	Filename     string  `json:"filename"`
	CollectionID string  `json:"collection_id"`
	ChunkIndex   int     `json:"chunk_index"`
	StartLine    int     `json:"start_line"`
	EndLine      int     `json:"end_line"`
	Text         string  `json:"text"`
	Score        float64 `json:"score"`
}

// KBSearchOptions configures a KB search query.
type KBSearchOptions struct {
	CollectionIDs []string
	MaxResults    int
	MinScore      float64
}

// KBChunkInsert is input for batch chunk insertion.
type KBChunkInsert struct {
	ChunkIndex int
	Text       string
	Hash       string
	StartLine  int
	EndLine    int
	Embedding  []float32
}

// KBStore manages knowledge base collections, documents, and search.
type KBStore interface {
	// Collections
	CreateCollection(ctx context.Context, agentID, name, description string) (*KBCollection, error)
	GetCollection(ctx context.Context, id string) (*KBCollection, error)
	ListCollections(ctx context.Context, agentID string) ([]KBCollection, error)
	UpdateCollection(ctx context.Context, id string, updates map[string]any) error
	DeleteCollection(ctx context.Context, id string) error

	// Documents
	CreateDocument(ctx context.Context, doc *KBDocument) error
	GetDocument(ctx context.Context, id string) (*KBDocument, error)
	ListDocuments(ctx context.Context, collectionID string) ([]KBDocument, error)
	UpdateDocumentStatus(ctx context.Context, id, status string, errMsg string, chunkCount, embeddedCount int) error
	DeleteDocument(ctx context.Context, id string) error

	// Chunks
	InsertChunks(ctx context.Context, documentID, agentID string, chunks []KBChunkInsert) error
	DeleteChunksByDocument(ctx context.Context, documentID string) error
	ListChunks(ctx context.Context, documentID string) ([]KBChunk, error)

	// Search
	Search(ctx context.Context, query string, agentID string, opts KBSearchOptions) ([]KBSearchResult, error)

	// Embedding
	SetEmbeddingProvider(provider EmbeddingProvider)
}
