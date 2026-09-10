package longcontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// GetEvidenceByTask returns all evidence rows for a task, ordered by chunk
// ordinal and then confidence descending.
func (r *PostgresRepository) GetEvidenceByTask(ctx context.Context, taskID string) ([]Evidence, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.task_id, e.chunk_id, e.claim, e.quote,
		       e.start_char, e.end_char, e.entities, e.relations,
		       e.confidence, e.verification_status, e.embedding, e.created_at
		FROM long_context_evidence e
		LEFT JOIN long_context_chunks c ON c.id = e.chunk_id
		WHERE e.task_id = $1
		ORDER BY c.ordinal, e.confidence DESC`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query evidence: %w", err)
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		var ev Evidence
		var embeddingRaw sql.NullString
		if err := rows.Scan(&ev.ID, &ev.TaskID, &ev.ChunkID, &ev.Claim, &ev.Quote,
			&ev.StartChar, &ev.EndChar, &ev.Entities, &ev.Relations,
			&ev.Confidence, &ev.VerificationStatus, &embeddingRaw, &ev.CreatedAt); err != nil {
			if err == sql.ErrNoRows {
				break
			}
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		if embeddingRaw.Valid && embeddingRaw.String != "" {
			if err := json.Unmarshal([]byte(embeddingRaw.String), &ev.Embedding); err != nil {
				return nil, fmt.Errorf("decode evidence embedding: %w", err)
			}
		}
		out = append(out, ev)
	}
	return out, nil
}

// ChunkForEvidence returns the original chunk content for a given chunk_id.
func (r *PostgresRepository) GetChunkContent(ctx context.Context, chunkID string) (string, error) {
	var content string
	err := r.db.QueryRowContext(ctx, `SELECT content FROM long_context_chunks WHERE id=$1`, chunkID).Scan(&content)
	if err != nil {
		return "", fmt.Errorf("get chunk content: %w", err)
	}
	return content, nil
}

// HybridRetrieval selects the top-k evidence items from a pool using a
// lightweight keyword-overlap scoring combined with the original confidence.
// This is intentionally simple (no vector store needed) and runs fast.
func HybridRetrieval(query string, evidence []Evidence, topK int) []Evidence {
	if topK <= 0 || len(evidence) == 0 {
		return evidence
	}
	queryTerms := tokenize(query)
	// Always deduplicate first — same claim+quote pair is kept once.
	seen := map[string]bool{}
	var deduped []Evidence
	for _, ev := range evidence {
		dedup := ev.Claim + "\x00" + ev.Quote
		if seen[dedup] {
			continue
		}
		seen[dedup] = true
		deduped = append(deduped, ev)
	}

	if len(queryTerms) == 0 {
		// No useful keywords — just return the highest-confidence items.
		out := make([]Evidence, len(deduped))
		copy(out, deduped)
		for i := 0; i < topK && i < len(out)-1; i++ {
			best := i
			for j := i + 1; j < len(out); j++ {
				if out[j].Confidence > out[best].Confidence {
					best = j
				}
			}
			out[i], out[best] = out[best], out[i]
		}
		n := topK
		if n > len(out) {
			n = len(out)
		}
		return out[:n]
	}

	type scored struct {
		ev    Evidence
		score float64
	}
	var items []scored
	for _, ev := range deduped {
		score := scoreEvidence(queryTerms, ev)
		items = append(items, scored{ev, score})
	}

	for i := 0; i < topK && i < len(items)-1; i++ {
		best := i
		for j := i + 1; j < len(items); j++ {
			if items[j].score > items[best].score {
				best = j
			}
		}
		items[i], items[best] = items[best], items[i]
	}
	n := topK
	if n > len(items) {
		n = len(items)
	}
	out := make([]Evidence, n)
	for i := 0; i < n; i++ {
		out[i] = items[i].ev
	}
	return out
}

func scoreEvidence(queryTerms []string, ev Evidence) float64 {
	text := strings.ToLower(ev.Claim + " " + ev.Quote)
	hits := 0
	for _, t := range queryTerms {
		if strings.Contains(text, t) {
			hits++
		}
	}
	boost := 1.0
	if len(queryTerms) > 0 {
		boost = 1.0 + float64(hits)/float64(len(queryTerms))
	}
	return ev.Confidence * boost
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == ' ' || r == '-' {
			return r
		}
		return ' '
	}, s)
	parts := strings.Fields(s)
	stop := map[string]bool{"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "of": true, "in": true, "to": true, "for": true,
		"and": true, "or": true, "it": true, "that": true, "this": true}
	var out []string
	for _, p := range parts {
		if len(p) <= 1 || stop[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// UpdateEvidenceEmbedding stores the embedding vector for an evidence row.
func (r *PostgresRepository) UpdateEvidenceEmbedding(ctx context.Context, evidenceID int64, vec []float64) error {
	raw, err := json.Marshal(vec)
	if err != nil {
		return fmt.Errorf("marshal evidence embedding: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `UPDATE long_context_evidence SET embedding=$1 WHERE id=$2`, string(raw), evidenceID)
	if err != nil {
		return fmt.Errorf("update evidence embedding: %w", err)
	}
	return nil
}
