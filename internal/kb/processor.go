package kb

import (
	"context"
	"log/slog"

	"github.com/nextlevelbuilder/goclaw/internal/media"
	"github.com/nextlevelbuilder/goclaw/internal/memory"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Processor handles document ingestion: extract -> chunk -> embed -> store.
type Processor struct {
	store    store.KBStore
	storage  media.Storage
	provider store.EmbeddingProvider
	maxChunk int
}

// NewProcessor creates a KB document processor.
func NewProcessor(kbStore store.KBStore, storage media.Storage, provider store.EmbeddingProvider) *Processor {
	return &Processor{
		store:    kbStore,
		storage:  storage,
		provider: provider,
		maxChunk: 1000,
	}
}

// ProcessDocument extracts text, chunks, embeds, and stores chunks.
// Called async after document upload.
func (p *Processor) ProcessDocument(ctx context.Context, docID string) error {
	doc, err := p.store.GetDocument(ctx, docID)
	if err != nil {
		return err
	}

	// Update status to processing
	p.store.UpdateDocumentStatus(ctx, docID, "processing", "", 0, 0)

	// Load file from storage
	filePath, err := p.storage.LoadPath(doc.StorageKey)
	if err != nil {
		p.store.UpdateDocumentStatus(ctx, docID, "error", "load failed: "+err.Error(), 0, 0)
		return err
	}

	// Extract text
	text, err := ExtractText(filePath, doc.MimeType)
	if err != nil {
		p.store.UpdateDocumentStatus(ctx, docID, "error", "extraction failed: "+err.Error(), 0, 0)
		return err
	}

	// Chunk text (reuse memory.ChunkText)
	chunks := memory.ChunkText(text, p.maxChunk)
	if len(chunks) == 0 {
		p.store.UpdateDocumentStatus(ctx, docID, "ready", "", 0, 0)
		return nil
	}

	// Generate embeddings (batch)
	var embeddings [][]float32
	if p.provider != nil {
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Text
		}
		embeddings, _ = p.provider.Embed(ctx, texts) // best-effort
	}

	// Build chunk inserts
	inserts := make([]store.KBChunkInsert, len(chunks))
	embeddedCount := 0
	for i, c := range chunks {
		inserts[i] = store.KBChunkInsert{
			ChunkIndex: i,
			Text:       c.Text,
			Hash:       memory.ContentHash(c.Text),
			StartLine:  c.StartLine,
			EndLine:    c.EndLine,
		}
		if embeddings != nil && i < len(embeddings) {
			inserts[i].Embedding = embeddings[i]
			embeddedCount++
		}
	}

	// Delete old chunks, insert new
	p.store.DeleteChunksByDocument(ctx, docID)
	if err := p.store.InsertChunks(ctx, docID, doc.AgentID, inserts); err != nil {
		p.store.UpdateDocumentStatus(ctx, docID, "error", "chunk insert failed: "+err.Error(), 0, 0)
		return err
	}

	// Update document status
	p.store.UpdateDocumentStatus(ctx, docID, "ready", "", len(chunks), embeddedCount)

	slog.Info("kb.document.processed",
		"doc_id", docID,
		"chunks", len(chunks),
		"embedded", embeddedCount,
	)
	return nil
}
