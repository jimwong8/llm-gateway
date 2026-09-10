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
	"llm-gateway/gateway/internal/memory"
)

const (
	recentWindow = int64(50)
	timeout      = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Println("rebuild success")
}

func run() error {
	conversationIDRaw := strings.TrimSpace(readConversationID())
	if conversationIDRaw == "" {
		return fmt.Errorf("conversation_id is required")
	}

	conversationID, err := strconv.ParseInt(conversationIDRaw, 10, 64)
	if err != nil || conversationID <= 0 {
		return fmt.Errorf("conversation_id must be a positive integer")
	}

	cfg := config.Load()
	rc := cache.NewRedis(cfg.RedisAddr, time.Duration(cfg.L1CacheTTLSeconds)*time.Second)
	store, err := memory.NewStore(cfg.PostgresDSN, rc)
	if err != nil {
		return fmt.Errorf("init memory store: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := rc.InvalidateConversationCache(ctx, conversationIDRaw); err != nil {
		return fmt.Errorf("invalidate cache: %w", err)
	}

	tenantID, userID, sessionID, err := lookupConversationIdentity(ctx, cfg.PostgresDSN, conversationID)
	if err != nil {
		return err
	}

	conversation, err := store.GetConversation(ctx, tenantID, userID, sessionID)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	if conversation == nil {
		return fmt.Errorf("conversation not found")
	}

	if err := rc.CacheConversationMeta(ctx, conversationIDRaw, cache.ConversationMeta{
		LastSeq:   conversation.LastSeq,
		UpdatedAt: conversation.UpdatedAt,
	}); err != nil {
		return fmt.Errorf("cache conversation meta: %w", err)
	}

	messages, err := store.GetMessages(ctx, sessionID, 0, int(recentWindow), "backward")
	if err != nil {
		return fmt.Errorf("get messages: %w", err)
	}

	recent := make([]cache.RecentMessage, 0, len(messages))
	for _, msg := range messages {
		recent = append(recent, cache.RecentMessage{
			Seq:     msg.Seq,
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	if len(recent) > 0 {
		if err := rc.CacheRecentMessages(ctx, conversationIDRaw, recent, recentWindow); err != nil {
			return fmt.Errorf("cache recent messages: %w", err)
		}
	}

	return nil
}

func readConversationID() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return os.Getenv("CONVERSATION_ID")
}

func lookupConversationIdentity(ctx context.Context, dsn string, conversationID int64) (string, string, string, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return "", "", "", fmt.Errorf("open postgres: %w", err)
	}
	defer db.Close()

	var tenantID sql.NullString
	var userID sql.NullString
	var sessionID string
	err = db.QueryRowContext(ctx, `
SELECT tenant_id, user_id, session_id
FROM conversations
WHERE id = $1
`, conversationID).Scan(&tenantID, &userID, &sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", fmt.Errorf("conversation not found")
		}
		return "", "", "", fmt.Errorf("query conversation: %w", err)
	}

	return tenantID.String, userID.String, sessionID, nil
}
