package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"llm-gateway/gateway/internal/cache"
	"llm-gateway/gateway/internal/config"
)

const (
	defaultTopN       = 100
	defaultRecentSize = 20
	timeout           = 30 * time.Second
)

type pgConversation struct {
	ID        int64
	SessionID string
	Status    string
	LastSeq   int64
}

type mismatchStats struct {
	Total                   int
	MetaMissingForActive    int
	MetaExistsForDeleted    int
	LastSeqMismatch         int
	RecentTailSeqMismatch   int
	RecentExistsForDeleted  int
	RecentTailMissingActive int
}

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	rc := cache.NewRedis(cfg.RedisAddr, time.Duration(cfg.L1CacheTTLSeconds)*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := rc.Ping(ctx); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	db, err := sql.Open("postgres", cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres ping failed: %w", err)
	}

	topN, err := readPositiveInt("TOP_N", defaultTopN)
	if err != nil {
		return err
	}
	recentLimit, err := readPositiveInt("RECENT_LIMIT", defaultRecentSize)
	if err != nil {
		return err
	}

	conversations, err := fetchTopConversations(ctx, db, topN)
	if err != nil {
		return err
	}

	stats := mismatchStats{Total: len(conversations)}
	for _, conv := range conversations {
		if err := reconcileConversation(ctx, rc, conv, recentLimit, &stats); err != nil {
			return err
		}
	}

	printSummary(topN, recentLimit, stats)
	return nil
}

func fetchTopConversations(ctx context.Context, db *sql.DB, topN int) ([]pgConversation, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id, session_id, COALESCE(status, 'active') AS status, last_seq
FROM conversations
ORDER BY updated_at DESC
LIMIT $1
`, topN)
	if err != nil {
		return nil, fmt.Errorf("query conversations: %w", err)
	}
	defer rows.Close()

	out := make([]pgConversation, 0, topN)
	for rows.Next() {
		var item pgConversation
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Status, &item.LastSeq); err != nil {
			return nil, fmt.Errorf("scan conversation row: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate conversations: %w", err)
	}
	return out, nil
}

func reconcileConversation(ctx context.Context, rc *cache.RedisCache, conv pgConversation, recentLimit int, stats *mismatchStats) error {
	conversationID := strconv.FormatInt(conv.ID, 10)
	meta, metaHit, err := rc.GetConversationMeta(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("get redis meta conversation_id=%d: %w", conv.ID, err)
	}

	recent, err := rc.GetRecentMessages(ctx, conversationID, int64(recentLimit))
	if err != nil {
		return fmt.Errorf("get redis recent conversation_id=%d: %w", conv.ID, err)
	}

	isDeleted := strings.EqualFold(strings.TrimSpace(conv.Status), "deleted")
	if isDeleted {
		if metaHit {
			stats.MetaExistsForDeleted++
			fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=deleted_meta_exists pg_status=%s redis_last_seq=%d\n", conv.ID, conv.SessionID, conv.Status, meta.LastSeq)
		}
		if len(recent) > 0 {
			stats.RecentExistsForDeleted++
			tail := recent[len(recent)-1]
			fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=deleted_recent_exists pg_status=%s redis_recent_count=%d redis_recent_tail_seq=%d\n", conv.ID, conv.SessionID, conv.Status, len(recent), tail.Seq)
		}
		return nil
	}

	if !metaHit {
		stats.MetaMissingForActive++
		fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=active_meta_missing pg_last_seq=%d\n", conv.ID, conv.SessionID, conv.LastSeq)
		return nil
	}

	if meta.LastSeq != conv.LastSeq {
		stats.LastSeqMismatch++
		fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=last_seq_mismatch pg_last_seq=%d redis_last_seq=%d\n", conv.ID, conv.SessionID, conv.LastSeq, meta.LastSeq)
	}

	if conv.LastSeq <= 0 {
		if len(recent) > 0 {
			tail := recent[len(recent)-1]
			stats.RecentTailSeqMismatch++
			fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=recent_tail_without_pg_seq pg_last_seq=%d redis_recent_tail_seq=%d\n", conv.ID, conv.SessionID, conv.LastSeq, tail.Seq)
		}
		return nil
	}

	if len(recent) == 0 {
		stats.RecentTailMissingActive++
		fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=active_recent_missing pg_last_seq=%d\n", conv.ID, conv.SessionID, conv.LastSeq)
		return nil
	}

	tail := recent[len(recent)-1]
	if tail.Seq != conv.LastSeq {
		stats.RecentTailSeqMismatch++
		fmt.Printf("[MISMATCH] conversation_id=%d session_id=%s type=recent_tail_seq_mismatch pg_last_seq=%d redis_recent_tail_seq=%d redis_recent_count=%d\n", conv.ID, conv.SessionID, conv.LastSeq, tail.Seq, len(recent))
	}

	return nil
}

func readPositiveInt(envKey string, fallback int) (int, error) {
	candidate := strings.TrimSpace(readArgOrEnv(envKey))
	if candidate == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(candidate)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", envKey)
	}
	return value, nil
}

func readArgOrEnv(envKey string) string {
	argPrefix := strings.ToLower(envKey) + "="
	for _, arg := range os.Args[1:] {
		normalized := strings.TrimSpace(arg)
		if strings.HasPrefix(strings.ToLower(normalized), argPrefix) {
			return strings.TrimSpace(normalized[len(argPrefix):])
		}
	}
	return os.Getenv(envKey)
}

func printSummary(topN, recentLimit int, stats mismatchStats) {
	totalMismatches := stats.MetaMissingForActive +
		stats.MetaExistsForDeleted +
		stats.LastSeqMismatch +
		stats.RecentTailSeqMismatch +
		stats.RecentExistsForDeleted +
		stats.RecentTailMissingActive

	fmt.Printf("[SUMMARY] reconciled=%d top_n=%d recent_limit=%d mismatches=%d\n", stats.Total, topN, recentLimit, totalMismatches)
	fmt.Printf("[SUMMARY] active_meta_missing=%d deleted_meta_exists=%d last_seq_mismatch=%d recent_tail_seq_mismatch=%d deleted_recent_exists=%d active_recent_missing=%d\n",
		stats.MetaMissingForActive,
		stats.MetaExistsForDeleted,
		stats.LastSeqMismatch,
		stats.RecentTailSeqMismatch,
		stats.RecentExistsForDeleted,
		stats.RecentTailMissingActive,
	)
}
