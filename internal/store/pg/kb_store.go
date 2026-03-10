package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// PGKBStore implements store.KBStore backed by Postgres.
type PGKBStore struct {
	db       *sql.DB
	provider store.EmbeddingProvider
}

func NewPGKBStore(db *sql.DB) *PGKBStore {
	return &PGKBStore{db: db}
}

func (s *PGKBStore) SetEmbeddingProvider(provider store.EmbeddingProvider) {
	s.provider = provider
}

// --- Collections ---

func (s *PGKBStore) CreateCollection(ctx context.Context, agentID, name, description string) (*store.KBCollection, error) {
	id := uuid.Must(uuid.NewV7())
	aid := mustParseUUID(agentID)
	now := nowUTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kb_collections (id, agent_id, name, description, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $5)`,
		id, aid, name, description, now)
	if err != nil {
		return nil, fmt.Errorf("create kb collection: %w", err)
	}

	return &store.KBCollection{
		ID:          id.String(),
		AgentID:     agentID,
		Name:        name,
		Description: description,
		DocCount:    0,
		Status:      "active",
		CreatedAt:   now.UnixMilli(),
		UpdatedAt:   now.UnixMilli(),
	}, nil
}

func (s *PGKBStore) GetCollection(ctx context.Context, id string) (*store.KBCollection, error) {
	uid := mustParseUUID(id)
	var c store.KBCollection
	var metaBytes []byte
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, agent_id, name, description, metadata, doc_count, status, created_at, updated_at
		 FROM kb_collections WHERE id = $1`, uid).Scan(
		&c.ID, &c.AgentID, &c.Name, &c.Description, &metaBytes,
		&c.DocCount, &c.Status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if metaBytes != nil {
		json.Unmarshal(metaBytes, &c.Metadata)
	}
	c.CreatedAt = createdAt.UnixMilli()
	c.UpdatedAt = updatedAt.UnixMilli()
	return &c, nil
}

func (s *PGKBStore) ListCollections(ctx context.Context, agentID string) ([]store.KBCollection, error) {
	aid := mustParseUUID(agentID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_id, name, description, doc_count, status, created_at, updated_at
		 FROM kb_collections WHERE agent_id = $1 ORDER BY created_at DESC`, aid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.KBCollection
	for rows.Next() {
		var c store.KBCollection
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&c.ID, &c.AgentID, &c.Name, &c.Description,
			&c.DocCount, &c.Status, &createdAt, &updatedAt); err != nil {
			continue
		}
		c.CreatedAt = createdAt.UnixMilli()
		c.UpdatedAt = updatedAt.UnixMilli()
		result = append(result, c)
	}
	return result, nil
}

func (s *PGKBStore) UpdateCollection(ctx context.Context, id string, updates map[string]any) error {
	uid := mustParseUUID(id)
	return execMapUpdate(ctx, s.db, "kb_collections", uid, updates)
}

func (s *PGKBStore) DeleteCollection(ctx context.Context, id string) error {
	uid := mustParseUUID(id)
	_, err := s.db.ExecContext(ctx, "DELETE FROM kb_collections WHERE id = $1", uid)
	return err
}

// --- Documents ---

func (s *PGKBStore) CreateDocument(ctx context.Context, doc *store.KBDocument) error {
	id := uuid.Must(uuid.NewV7())
	aid := mustParseUUID(doc.AgentID)
	cid := mustParseUUID(doc.CollectionID)
	now := nowUTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kb_documents (id, collection_id, agent_id, filename, mime_type, file_size,
		 storage_key, content_hash, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)`,
		id, cid, aid, doc.Filename, doc.MimeType, doc.FileSize,
		doc.StorageKey, doc.ContentHash, "pending", now)
	if err != nil {
		return fmt.Errorf("create kb document: %w", err)
	}

	doc.ID = id.String()
	doc.Status = "pending"
	doc.CreatedAt = now.UnixMilli()
	doc.UpdatedAt = now.UnixMilli()

	// Increment collection doc_count
	s.db.ExecContext(ctx,
		"UPDATE kb_collections SET doc_count = doc_count + 1, updated_at = $1 WHERE id = $2",
		now, cid)

	return nil
}

func (s *PGKBStore) GetDocument(ctx context.Context, id string) (*store.KBDocument, error) {
	uid := mustParseUUID(id)
	var d store.KBDocument
	var errMsg *string
	var createdAt, updatedAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT id, collection_id, agent_id, filename, mime_type, file_size,
		 storage_key, content_hash, version, status, error_message,
		 chunk_count, embedded_count, created_at, updated_at
		 FROM kb_documents WHERE id = $1`, uid).Scan(
		&d.ID, &d.CollectionID, &d.AgentID, &d.Filename, &d.MimeType, &d.FileSize,
		&d.StorageKey, &d.ContentHash, &d.Version, &d.Status, &errMsg,
		&d.ChunkCount, &d.EmbeddedCount, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	d.ErrorMessage = derefStr(errMsg)
	d.CreatedAt = createdAt.UnixMilli()
	d.UpdatedAt = updatedAt.UnixMilli()
	return &d, nil
}

func (s *PGKBStore) ListDocuments(ctx context.Context, collectionID string) ([]store.KBDocument, error) {
	cid := mustParseUUID(collectionID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, collection_id, agent_id, filename, mime_type, file_size,
		 content_hash, version, status, error_message, chunk_count, embedded_count,
		 created_at, updated_at
		 FROM kb_documents WHERE collection_id = $1 ORDER BY created_at DESC`, cid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.KBDocument
	for rows.Next() {
		var d store.KBDocument
		var errMsg *string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&d.ID, &d.CollectionID, &d.AgentID, &d.Filename,
			&d.MimeType, &d.FileSize, &d.ContentHash, &d.Version, &d.Status,
			&errMsg, &d.ChunkCount, &d.EmbeddedCount, &createdAt, &updatedAt); err != nil {
			continue
		}
		d.ErrorMessage = derefStr(errMsg)
		d.CreatedAt = createdAt.UnixMilli()
		d.UpdatedAt = updatedAt.UnixMilli()
		result = append(result, d)
	}
	return result, nil
}

func (s *PGKBStore) UpdateDocumentStatus(ctx context.Context, id, status string, errMsg string, chunkCount, embeddedCount int) error {
	uid := mustParseUUID(id)
	now := nowUTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE kb_documents SET status = $1, error_message = $2,
		 chunk_count = $3, embedded_count = $4, updated_at = $5
		 WHERE id = $6`,
		status, nilStr(errMsg), chunkCount, embeddedCount, now, uid)
	return err
}

func (s *PGKBStore) DeleteDocument(ctx context.Context, id string) error {
	uid := mustParseUUID(id)

	// Get collection_id before deleting
	var cid uuid.UUID
	s.db.QueryRowContext(ctx, "SELECT collection_id FROM kb_documents WHERE id = $1", uid).Scan(&cid)

	_, err := s.db.ExecContext(ctx, "DELETE FROM kb_documents WHERE id = $1", uid)
	if err != nil {
		return err
	}

	// Decrement collection doc_count
	if cid != uuid.Nil {
		s.db.ExecContext(ctx,
			"UPDATE kb_collections SET doc_count = GREATEST(doc_count - 1, 0), updated_at = $1 WHERE id = $2",
			nowUTC(), cid)
	}
	return nil
}

// --- Chunks ---

func (s *PGKBStore) InsertChunks(ctx context.Context, documentID, agentID string, chunks []store.KBChunkInsert) error {
	if len(chunks) == 0 {
		return nil
	}
	did := mustParseUUID(documentID)
	aid := mustParseUUID(agentID)
	now := nowUTC()

	for _, c := range chunks {
		chunkID := uuid.Must(uuid.NewV7())
		if c.Embedding != nil {
			s.db.ExecContext(ctx,
				`INSERT INTO kb_chunks (id, document_id, agent_id, chunk_index, text, hash,
				 start_line, end_line, embedding, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::vector, $10, $10)`,
				chunkID, did, aid, c.ChunkIndex, c.Text, c.Hash,
				c.StartLine, c.EndLine, vectorToString(c.Embedding), now)
		} else {
			s.db.ExecContext(ctx,
				`INSERT INTO kb_chunks (id, document_id, agent_id, chunk_index, text, hash,
				 start_line, end_line, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)`,
				chunkID, did, aid, c.ChunkIndex, c.Text, c.Hash,
				c.StartLine, c.EndLine, now)
		}
	}
	return nil
}

func (s *PGKBStore) DeleteChunksByDocument(ctx context.Context, documentID string) error {
	did := mustParseUUID(documentID)
	_, err := s.db.ExecContext(ctx, "DELETE FROM kb_chunks WHERE document_id = $1", did)
	return err
}

func (s *PGKBStore) ListChunks(ctx context.Context, documentID string) ([]store.KBChunk, error) {
	did := mustParseUUID(documentID)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, document_id, chunk_index, text, start_line, end_line,
		 (embedding IS NOT NULL) AS has_embedding
		 FROM kb_chunks WHERE document_id = $1 ORDER BY chunk_index`, did)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []store.KBChunk
	for rows.Next() {
		var c store.KBChunk
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Text,
			&c.StartLine, &c.EndLine, &c.HasEmbedding); err != nil {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}
