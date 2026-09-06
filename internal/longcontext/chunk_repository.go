package longcontext

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ChunkRepository interface {
	ClaimChunk(context.Context, string, string, time.Duration) (*Chunk, error)
	SaveMapResult(context.Context, string, string, string, string, MapResult) error
	ReleaseChunk(context.Context, string, string) error
}

func (r *PostgresRepository) ClaimChunk(ctx context.Context, taskID, worker string, lease time.Duration) (*Chunk, error) {
	if lease <= 0 {
		lease = time.Minute
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var c Chunk
	var raw []byte
	// Claim either a 'pending' chunk, or a 'mapping' chunk whose lease has
	// expired (covers a worker that died or an upstream call that hung forever).
	err = tx.QueryRowContext(ctx, `
		SELECT id,task_id,ordinal,start_char,end_char,start_token,end_token,content,content_sha256,status,map_attempts,map_result,map_model,map_provider,created_at,updated_at
		FROM long_context_chunks
		WHERE task_id=$1 AND (status='pending' OR (status='mapping' AND map_lease_until IS NOT NULL AND map_lease_until < NOW()))
		ORDER BY ordinal
		FOR UPDATE SKIP LOCKED LIMIT 1`, taskID).
		Scan(&c.ID, &c.TaskID, &c.Ordinal, &c.StartChar, &c.EndChar, &c.StartToken, &c.EndToken, &c.Content, &c.ContentHash, &c.Status, &c.MapAttempts, &raw, &c.MapModel, &c.MapProvider, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) > 0 {
		c.MapResult = json.RawMessage(raw)
	}
	if _, err = tx.ExecContext(ctx, `UPDATE long_context_chunks SET status='mapping',map_attempts=map_attempts+1,map_lease_until=NOW()+$1::interval,updated_at=NOW() WHERE id=$2`, fmt.Sprintf("%f seconds", lease.Seconds()), c.ID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	c.Status = "mapping"
	c.MapAttempts++
	return &c, nil
}

func (r *PostgresRepository) ReleaseChunk(ctx context.Context, chunkID, worker string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE long_context_chunks SET status='pending',map_lease_until=NULL,updated_at=NOW() WHERE id=$1`, chunkID)
	return err
}

func (r *PostgresRepository) SaveMapResult(ctx context.Context, chunkID, taskID, model, provider string, result MapResult) error {
	var c Chunk
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT id,task_id,ordinal,start_char,end_char,start_token,end_token,content,content_sha256,status,map_attempts,map_result,map_model,map_provider,created_at,updated_at FROM long_context_chunks WHERE id=$1 AND task_id=$2`, chunkID, taskID).Scan(&c.ID, &c.TaskID, &c.Ordinal, &c.StartChar, &c.EndChar, &c.StartToken, &c.EndToken, &c.Content, &c.ContentHash, &c.Status, &c.MapAttempts, &raw, &c.MapModel, &c.MapProvider, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return err
	}
	if err := ValidateMapResult(result, c); err != nil {
		return err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE long_context_chunks SET status='mapped',map_result=$1,map_model=$2,map_provider=$3,map_lease_until=NULL,updated_at=NOW() WHERE id=$4 AND task_id=$5 AND status='mapping'`, encoded, model, provider, chunkID, taskID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return fmt.Errorf("chunk map result compare-and-swap failed")
	}
	for _, claim := range result.Claims {
		entities := claim.Entities
		if len(entities) == 0 {
			entities = json.RawMessage(`[]`)
		}
		relations := claim.Relations
		if len(relations) == 0 {
			relations = json.RawMessage(`[]`)
		}
		if _, err := r.db.ExecContext(ctx, `INSERT INTO long_context_evidence (task_id,chunk_id,claim,quote,start_char,end_char,entities,relations,confidence,verification_status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'quote_verified')`, taskID, chunkID, claim.Claim, claim.Quote, claim.StartChar, claim.EndChar, entities, relations, claim.Confidence); err != nil {
			return fmt.Errorf("save evidence: %w", err)
		}
	}
	return nil
}
