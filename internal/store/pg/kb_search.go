package pg

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Search performs hybrid search (FTS + vector) over kb_chunks scoped to an agent.
func (s *PGKBStore) Search(ctx context.Context, query string, agentID string, opts store.KBSearchOptions) ([]store.KBSearchResult, error) {
	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	aid := mustParseUUID(agentID)

	// FTS search
	ftsResults, err := s.kbFTSSearch(ctx, query, aid, opts.CollectionIDs, maxResults*2)
	if err != nil {
		return nil, err
	}

	// Vector search if provider available
	var vecResults []kbScoredChunk
	if s.provider != nil {
		embeddings, err := s.provider.Embed(ctx, []string{query})
		if err == nil && len(embeddings) > 0 {
			vecResults, _ = s.kbVectorSearch(ctx, embeddings[0], aid, opts.CollectionIDs, maxResults*2)
		}
	}

	// Merge with weights
	textW, vecW := 0.3, 0.7
	if len(ftsResults) == 0 && len(vecResults) > 0 {
		textW, vecW = 0, 1.0
	} else if len(vecResults) == 0 && len(ftsResults) > 0 {
		textW, vecW = 1.0, 0
	}
	merged := kbHybridMerge(ftsResults, vecResults, textW, vecW)

	// Apply min score filter and limit
	var filtered []store.KBSearchResult
	for _, m := range merged {
		if opts.MinScore > 0 && m.Score < opts.MinScore {
			continue
		}
		filtered = append(filtered, m)
		if len(filtered) >= maxResults {
			break
		}
	}
	return filtered, nil
}

type kbScoredChunk struct {
	DocumentID   string
	Filename     string
	CollectionID string
	ChunkIndex   int
	StartLine    int
	EndLine      int
	Text         string
	Score        float64
}

func (s *PGKBStore) kbFTSSearch(ctx context.Context, query string, agentID interface{}, collectionIDs []string, limit int) ([]kbScoredChunk, error) {
	var q string
	var args []interface{}

	if len(collectionIDs) > 0 {
		q = `SELECT c.document_id, d.filename, d.collection_id, c.chunk_index,
				c.start_line, c.end_line, c.text,
				ts_rank(c.tsv, plainto_tsquery('simple', $1)) AS score
			FROM kb_chunks c
			JOIN kb_documents d ON d.id = c.document_id
			WHERE c.agent_id = $2 AND c.tsv @@ plainto_tsquery('simple', $3)
			AND d.collection_id = ANY($4::uuid[])
			ORDER BY score DESC LIMIT $5`
		args = []interface{}{query, agentID, query, pqUUIDArray(collectionIDs), limit}
	} else {
		q = `SELECT c.document_id, d.filename, d.collection_id, c.chunk_index,
				c.start_line, c.end_line, c.text,
				ts_rank(c.tsv, plainto_tsquery('simple', $1)) AS score
			FROM kb_chunks c
			JOIN kb_documents d ON d.id = c.document_id
			WHERE c.agent_id = $2 AND c.tsv @@ plainto_tsquery('simple', $3)
			ORDER BY score DESC LIMIT $4`
		args = []interface{}{query, agentID, query, limit}
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []kbScoredChunk
	for rows.Next() {
		var r kbScoredChunk
		rows.Scan(&r.DocumentID, &r.Filename, &r.CollectionID, &r.ChunkIndex,
			&r.StartLine, &r.EndLine, &r.Text, &r.Score)
		results = append(results, r)
	}
	return results, nil
}

func (s *PGKBStore) kbVectorSearch(ctx context.Context, embedding []float32, agentID interface{}, collectionIDs []string, limit int) ([]kbScoredChunk, error) {
	vecStr := vectorToString(embedding)

	var q string
	var args []interface{}

	if len(collectionIDs) > 0 {
		q = `SELECT c.document_id, d.filename, d.collection_id, c.chunk_index,
				c.start_line, c.end_line, c.text,
				1 - (c.embedding <=> $1::vector) AS score
			FROM kb_chunks c
			JOIN kb_documents d ON d.id = c.document_id
			WHERE c.agent_id = $2 AND c.embedding IS NOT NULL
			AND d.collection_id = ANY($3::uuid[])
			ORDER BY c.embedding <=> $4::vector LIMIT $5`
		args = []interface{}{vecStr, agentID, pqUUIDArray(collectionIDs), vecStr, limit}
	} else {
		q = `SELECT c.document_id, d.filename, d.collection_id, c.chunk_index,
				c.start_line, c.end_line, c.text,
				1 - (c.embedding <=> $1::vector) AS score
			FROM kb_chunks c
			JOIN kb_documents d ON d.id = c.document_id
			WHERE c.agent_id = $2 AND c.embedding IS NOT NULL
			ORDER BY c.embedding <=> $3::vector LIMIT $4`
		args = []interface{}{vecStr, agentID, vecStr, limit}
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []kbScoredChunk
	for rows.Next() {
		var r kbScoredChunk
		rows.Scan(&r.DocumentID, &r.Filename, &r.CollectionID, &r.ChunkIndex,
			&r.StartLine, &r.EndLine, &r.Text, &r.Score)
		results = append(results, r)
	}
	return results, nil
}

// kbHybridMerge combines FTS and vector results with weighted scoring.
func kbHybridMerge(fts, vec []kbScoredChunk, textWeight, vectorWeight float64) []store.KBSearchResult {
	type key struct {
		DocumentID string
		ChunkIndex int
	}
	seen := make(map[key]*store.KBSearchResult)

	addResult := func(r kbScoredChunk, weight float64) {
		k := key{r.DocumentID, r.ChunkIndex}
		score := r.Score * weight

		if existing, ok := seen[k]; ok {
			existing.Score += score
		} else {
			seen[k] = &store.KBSearchResult{
				DocumentID:   r.DocumentID,
				Filename:     r.Filename,
				CollectionID: r.CollectionID,
				ChunkIndex:   r.ChunkIndex,
				StartLine:    r.StartLine,
				EndLine:      r.EndLine,
				Text:         r.Text,
				Score:        score,
			}
		}
	}

	for _, r := range fts {
		addResult(r, textWeight)
	}
	for _, r := range vec {
		addResult(r, vectorWeight)
	}

	results := make([]store.KBSearchResult, 0, len(seen))
	for _, r := range seen {
		results = append(results, *r)
	}

	// Sort descending by score
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// pqUUIDArray formats a string slice as a PostgreSQL UUID array literal.
// Only valid UUIDs are included; invalid strings are silently skipped.
func pqUUIDArray(ids []string) string {
	if len(ids) == 0 {
		return "{}"
	}
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, err := uuid.Parse(id); err == nil {
			valid = append(valid, id)
		}
	}
	if len(valid) == 0 {
		return "{}"
	}
	return "{" + strings.Join(valid, ",") + "}"
}
