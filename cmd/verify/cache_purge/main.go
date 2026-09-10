package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"llm-gateway/gateway/internal/config"
)

const (
	defaultScanCount = int64(200)
	timeout          = 30 * time.Second
	matchPattern     = "conv:*"
)

func main() {
	if err := run(); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	fmt.Println("cache purge success")
}

func run() error {
	cfg := config.Load()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	scanCount, err := readPositiveInt("SCAN_COUNT", defaultScanCount)
	if err != nil {
		return err
	}

	deleted, err := purgeByPrefix(ctx, rdb, matchPattern, scanCount)
	if err != nil {
		return err
	}

	fmt.Printf("purged keys=%d pattern=%s\n", deleted, matchPattern)
	return nil
}

func purgeByPrefix(ctx context.Context, rdb *redis.Client, pattern string, scanCount int64) (int64, error) {
	var (
		cursor       uint64
		totalDeleted int64
	)

	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, scanCount).Result()
		if err != nil {
			return totalDeleted, fmt.Errorf("scan keys pattern=%s: %w", pattern, err)
		}

		if len(keys) > 0 {
			deleted, err := rdb.Del(ctx, keys...).Result()
			if err != nil {
				return totalDeleted, fmt.Errorf("delete keys pattern=%s: %w", pattern, err)
			}
			totalDeleted += deleted
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return totalDeleted, nil
}

func readPositiveInt(envKey string, fallback int64) (int64, error) {
	candidate := strings.TrimSpace(readArgOrEnv(envKey))
	if candidate == "" {
		return fallback, nil
	}

	value, err := strconv.ParseInt(candidate, 10, 64)
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
