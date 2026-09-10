package memory

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
)

type HybridSearcher struct {
	db *sql.DB
}

func NewHybridSearcher(db *sql.DB) *HybridSearcher {
	return &HybridSearcher{db: db}
}

type HybridSearchResult struct {
	ID       int64   `json:"id"`
	Content  string  `json:"content"`
	Score    float64 `json:"score"`
	Rank     int     `json:"rank"`
	Source   string  `json:"source"`
}

type hybridDoc struct {
	ID      int64
	Content string
	BM25    float64
	Vector  float64
}

const rrfK = 60.0

func (s *HybridSearcher) Search(ctx context.Context, userID int64, query string, limit int) ([]HybridSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	bm25Results, err := s.bm25Search(ctx, userID, query, limit*2)
	if err != nil {
		return nil, fmt.Errorf("bm25 search: %w", err)
	}

	vectorResults, err := s.vectorSearch(ctx, userID, query, limit*2)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	merged := s.reciprocalRankFusion(bm25Results, vectorResults, limit)

	return merged, nil
}

func (s *HybridSearcher) bm25Search(ctx context.Context, userID int64, query string, limit int) ([]hybridDoc, error) {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	tsQuery := strings.Join(tokens, " & ")
	rows, err := s.db.QueryContext(ctx, `
SELECT id, content,
  ts_rank(to_tsvector('simple', content), to_tsquery('simple', $2)) AS rank
FROM memory_atoms
WHERE user_id = $1 AND to_tsvector('simple', content) @@ to_tsquery('simple', $2)
ORDER BY rank DESC LIMIT $3`,
		userID, tsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []hybridDoc
	for rows.Next() {
		var d hybridDoc
		if err := rows.Scan(&d.ID, &d.Content, &d.BM25); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (s *HybridSearcher) vectorSearch(ctx context.Context, userID int64, query string, limit int) ([]hybridDoc, error) {
	embedding, err := s.getEmbedding(ctx, query)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
SELECT id, content,
  1 - (embedding <=> $2::vector) AS similarity
FROM memory_atoms
WHERE user_id = $1 AND embedding IS NOT NULL
ORDER BY embedding <=> $2::vector LIMIT $3`,
		userID, pgVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []hybridDoc
	for rows.Next() {
		var d hybridDoc
		if err := rows.Scan(&d.ID, &d.Content, &d.Vector); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}

func (s *HybridSearcher) reciprocalRankFusion(bm25Docs, vectorDocs []hybridDoc, limit int) []HybridSearchResult {
	docMap := make(map[int64]*hybridDoc)

	for i := range bm25Docs {
		d := bm25Docs[i]
		docMap[d.ID] = &d
		rrfScore := 1.0 / (rrfK + float64(i+1))
		d.BM25 = rrfScore
	}

	for i := range vectorDocs {
		d := vectorDocs[i]
		if existing, ok := docMap[d.ID]; ok {
			existing.Vector = 1.0 / (rrfK + float64(i+1))
		} else {
			d.Vector = 1.0 / (rrfK + float64(i+1))
			docMap[d.ID] = &d
		}
	}

	type scored struct {
		id    int64
		score float64
		doc   *hybridDoc
	}
	var scoredDocs []scored
	for id, d := range docMap {
		totalScore := d.BM25 + d.Vector
		scoredDocs = append(scoredDocs, scored{id: id, score: totalScore, doc: d})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	if len(scoredDocs) > limit {
		scoredDocs = scoredDocs[:limit]
	}

	results := make([]HybridSearchResult, 0, len(scoredDocs))
	for i, sd := range scoredDocs {
		source := "hybrid"
		if sd.doc.BM25 > 0 && sd.doc.Vector == 0 {
			source = "bm25"
		} else if sd.doc.Vector > 0 && sd.doc.BM25 == 0 {
			source = "vector"
		}
		results = append(results, HybridSearchResult{
			ID:     sd.id,
			Content: sd.doc.Content,
			Score:  math.Round(sd.score*10000) / 10000,
			Rank:   i + 1,
			Source: source,
		})
	}

	return results
}

func (s *HybridSearcher) getEmbedding(ctx context.Context, text string) ([]float32, error) {
	placeholder := make([]float32, 384)
	for i := range placeholder {
		placeholder[i] = float32(len(text)%100) / 100.0
	}
	return placeholder, nil
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	for _, t := range strings.Fields(text) {
		t = strings.Trim(t, ".,!?;:\"'()[]{}")
		if len(t) > 1 {
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func pgVector(v []float32) string {
	s := "["
	for i, f := range v {
		if i > 0 {
			s += ","
		}
		s += fmt.Sprintf("%f", f)
	}
	return s + "]"
}
