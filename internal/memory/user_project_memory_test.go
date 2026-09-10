package memory

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"llm-gateway/gateway/internal/providers"
)

func memoryTestPostgresDSN(t *testing.T) string {
	t.Helper()

	if dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("MEMORY_TEST_POSTGRES_DSN")); dsn != "" {
		return dsn
	}
	t.Skip("skip integration test: set POSTGRES_DSN or MEMORY_TEST_POSTGRES_DSN")
	return ""
}

func TestStoreUserPreferencesUpsertAndGetIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-pref-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertUserPreference(ctx, UserPreference{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "language",
		Value:      "zh-CN",
		SourceText: "以后都用中文回答",
	}); err != nil {
		t.Fatalf("UpsertUserPreference(language) error = %v", err)
	}
	if err := store.UpsertUserPreference(ctx, UserPreference{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "verbosity",
		Value:      "low",
		SourceText: "回答简洁一点",
	}); err != nil {
		t.Fatalf("UpsertUserPreference(verbosity) error = %v", err)
	}
	if err := store.UpsertUserPreference(ctx, UserPreference{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "Language",
		Value:      "en-US",
		SourceText: "后续改成英文",
	}); err != nil {
		t.Fatalf("UpsertUserPreference(language update) error = %v", err)
	}

	prefs, err := store.GetUserPreferences(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetUserPreferences() error = %v", err)
	}
	if len(prefs) != 2 {
		t.Fatalf("expected 2 preferences, got %d (%#v)", len(prefs), prefs)
	}
	if prefs[0].Key != "language" || prefs[0].Value != "en-US" {
		t.Fatalf("expected language=en-US at index 0, got %s=%s", prefs[0].Key, prefs[0].Value)
	}
	if prefs[0].SourceText != "后续改成英文" {
		t.Fatalf("expected updated source text, got %q", prefs[0].SourceText)
	}
	if prefs[1].Key != "verbosity" || prefs[1].Value != "low" {
		t.Fatalf("expected verbosity=low at index 1, got %s=%s", prefs[1].Key, prefs[1].Value)
	}
}

func TestStoreProjectFactsUpsertAndGetIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-fact-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "pg_truth",
		Value:      "PG is Truth",
		SourceText: "已确认：PG is Truth",
	}); err != nil {
		t.Fatalf("UpsertProjectFact(pg_truth) error = %v", err)
	}
	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "redis_role",
		Value:      "Redis 只做热层",
		SourceText: "最终决定：Redis 只做热层",
	}); err != nil {
		t.Fatalf("UpsertProjectFact(redis_role) error = %v", err)
	}
	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "PG_TRUTH",
		Value:      "PostgreSQL is source of truth",
		SourceText: "已确认：PostgreSQL is source of truth",
	}); err != nil {
		t.Fatalf("UpsertProjectFact(pg_truth update) error = %v", err)
	}

	facts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 facts, got %d (%#v)", len(facts), facts)
	}
	if facts[0].Key != "pg_truth" || facts[0].Value != "PostgreSQL is source of truth" {
		t.Fatalf("expected pg_truth updated at index 0, got %s=%s", facts[0].Key, facts[0].Value)
	}
	if facts[1].Key != "redis_role" || facts[1].Value != "Redis 只做热层" {
		t.Fatalf("expected redis_role at index 1, got %s=%s", facts[1].Key, facts[1].Value)
	}
}

func TestStoreProjectFactsSupersedeGovernanceIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-fact-govern-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	factKey := "runtime_policy"

	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              factKey,
		Value:            "use pg as source of truth",
		SourceText:       "已确认：use pg as source of truth",
		SourceMessageSeq: 11,
	}); err != nil {
		t.Fatalf("UpsertProjectFact(first) error = %v", err)
	}

	verifiedAt := time.Now().UTC().Add(2 * time.Second)
	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              factKey,
		Value:            "postgres is source of truth",
		SourceText:       "已确认：postgres is source of truth",
		SourceMessageSeq: 22,
		LastVerifiedAt:   verifiedAt,
	}); err != nil {
		t.Fatalf("UpsertProjectFact(second) error = %v", err)
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 1 {
		t.Fatalf("expected 1 active fact, got %d (%#v)", len(activeFacts), activeFacts)
	}
	active := activeFacts[0]
	if active.Status != "active" {
		t.Fatalf("expected active status, got %q", active.Status)
	}
	if active.Value != "postgres is source of truth" {
		t.Fatalf("expected latest active value, got %q", active.Value)
	}
	if active.SourceMessageSeq != 22 {
		t.Fatalf("expected source_message_seq=22, got %d", active.SourceMessageSeq)
	}
	if active.LastVerifiedAt.IsZero() {
		t.Fatalf("expected active last_verified_at not zero")
	}
	if active.LastVerifiedAt.Before(verifiedAt.Add(-2 * time.Second)) {
		t.Fatalf("expected active last_verified_at close to provided time, got %s", active.LastVerifiedAt)
	}

	allFacts, err := store.getProjectFacts(ctx, tenantID, userID, true)
	if err != nil {
		t.Fatalf("getProjectFacts(includeSuperseded=true) error = %v", err)
	}
	if len(allFacts) != 2 {
		t.Fatalf("expected 2 facts including superseded, got %d (%#v)", len(allFacts), allFacts)
	}

	var superseded *ProjectFact
	var latest *ProjectFact
	for i := range allFacts {
		fact := &allFacts[i]
		switch fact.Status {
		case "superseded":
			superseded = fact
		case "active":
			latest = fact
		}
	}
	if superseded == nil || latest == nil {
		t.Fatalf("expected both superseded and active facts, got %#v", allFacts)
	}
	if superseded.SupersededBy == nil {
		t.Fatalf("expected superseded fact to reference new fact id")
	}
	if *superseded.SupersededBy != latest.ID {
		t.Fatalf("expected superseded_by=%d, got %d", latest.ID, *superseded.SupersededBy)
	}
	if superseded.SourceMessageSeq != 11 {
		t.Fatalf("expected superseded source_message_seq=11, got %d", superseded.SourceMessageSeq)
	}
	if superseded.LastVerifiedAt.IsZero() {
		t.Fatalf("expected superseded last_verified_at not zero")
	}
}

func TestStoreUserPreferenceAndProjectFactScopeIsolationIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	tenantA := "tenant-a-" + suffix
	tenantB := "tenant-b-" + suffix
	userA := "user-a"
	userB := "user-b"

	if err := store.UpsertUserPreference(ctx, UserPreference{TenantID: tenantA, UserID: userA, Key: "language", Value: "zh-CN", SourceText: "以后都用中文"}); err != nil {
		t.Fatalf("UpsertUserPreference tenantA/userA error = %v", err)
	}
	if err := store.UpsertUserPreference(ctx, UserPreference{TenantID: tenantB, UserID: userA, Key: "language", Value: "en-US", SourceText: "always english"}); err != nil {
		t.Fatalf("UpsertUserPreference tenantB/userA error = %v", err)
	}

	prefsTenantA, err := store.GetUserPreferences(ctx, tenantA, userA)
	if err != nil {
		t.Fatalf("GetUserPreferences tenantA/userA error = %v", err)
	}
	if len(prefsTenantA) != 1 || prefsTenantA[0].Value != "zh-CN" {
		t.Fatalf("expected isolated prefs for tenantA/userA, got %#v", prefsTenantA)
	}

	prefsTenantB, err := store.GetUserPreferences(ctx, tenantB, userA)
	if err != nil {
		t.Fatalf("GetUserPreferences tenantB/userA error = %v", err)
	}
	if len(prefsTenantB) != 1 || prefsTenantB[0].Value != "en-US" {
		t.Fatalf("expected isolated prefs for tenantB/userA, got %#v", prefsTenantB)
	}

	if err := store.UpsertProjectFact(ctx, ProjectFact{TenantID: tenantA, UserID: userA, Key: "pg_truth", Value: "PG is Truth", SourceText: "已确认：PG is Truth"}); err != nil {
		t.Fatalf("UpsertProjectFact tenantA/userA error = %v", err)
	}
	if err := store.UpsertProjectFact(ctx, ProjectFact{TenantID: tenantA, UserID: userB, Key: "pg_truth", Value: "PG only for analytics", SourceText: "已确认：PG only for analytics"}); err != nil {
		t.Fatalf("UpsertProjectFact tenantA/userB error = %v", err)
	}

	factsUserA, err := store.GetProjectFacts(ctx, tenantA, userA)
	if err != nil {
		t.Fatalf("GetProjectFacts tenantA/userA error = %v", err)
	}
	if len(factsUserA) != 1 || factsUserA[0].Value != "PG is Truth" {
		t.Fatalf("expected isolated facts for tenantA/userA, got %#v", factsUserA)
	}

	factsUserB, err := store.GetProjectFacts(ctx, tenantA, userB)
	if err != nil {
		t.Fatalf("GetProjectFacts tenantA/userB error = %v", err)
	}
	if len(factsUserB) != 1 || factsUserB[0].Value != "PG only for analytics" {
		t.Fatalf("expected isolated facts for tenantA/userB, got %#v", factsUserB)
	}
}

func TestStoreProjectFactsConflictChainKeepsSingleActiveIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-fact-chain-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	factKey := "pg_truth"

	inputs := []ProjectFact{
		{
			TenantID:         tenantID,
			UserID:           userID,
			Key:              factKey,
			Value:            "PG is Truth",
			SourceText:       "已确认：PG is Truth",
			SourceMessageSeq: 10,
			LastVerifiedAt:   time.Now().UTC().Add(-240 * time.Hour),
		},
		{
			TenantID:         tenantID,
			UserID:           userID,
			Key:              factKey,
			Value:            "Postgres is source of truth",
			SourceText:       "最终决定：Postgres is source of truth",
			SourceMessageSeq: 20,
			LastVerifiedAt:   time.Now().UTC().Add(-48 * time.Hour),
		},
		{
			TenantID:         tenantID,
			UserID:           userID,
			Key:              factKey,
			Value:            "PostgreSQL is source of truth (final)",
			SourceText:       "结论：PostgreSQL is source of truth (final)",
			SourceMessageSeq: 30,
			LastVerifiedAt:   time.Now().UTC(),
		},
	}
	for i := range inputs {
		if err := store.UpsertProjectFact(ctx, inputs[i]); err != nil {
			t.Fatalf("UpsertProjectFact(version=%d) error = %v", i+1, err)
		}
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 1 {
		t.Fatalf("expected 1 active fact after conflict chain upserts, got %d (%#v)", len(activeFacts), activeFacts)
	}
	if activeFacts[0].Status != "active" {
		t.Fatalf("expected active status, got %q", activeFacts[0].Status)
	}
	if activeFacts[0].Value != "PostgreSQL is source of truth (final)" {
		t.Fatalf("expected latest value as active fact, got %q", activeFacts[0].Value)
	}
	if activeFacts[0].SourceMessageSeq != 30 {
		t.Fatalf("expected latest source_message_seq=30, got %d", activeFacts[0].SourceMessageSeq)
	}

	allFacts, err := store.getProjectFacts(ctx, tenantID, userID, true)
	if err != nil {
		t.Fatalf("getProjectFacts(includeSuperseded=true) error = %v", err)
	}
	if len(allFacts) != 3 {
		t.Fatalf("expected 3 total facts including superseded history, got %d (%#v)", len(allFacts), allFacts)
	}

	var latest *ProjectFact
	byID := map[int64]*ProjectFact{}
	supersededCount := 0
	for i := range allFacts {
		fact := &allFacts[i]
		if fact.Status == "active" {
			latest = fact
		}
		if fact.Status == "superseded" {
			supersededCount++
		}
		byID[fact.ID] = fact
	}
	if latest == nil {
		t.Fatalf("expected one active fact in all facts, got %#v", allFacts)
	}
	if supersededCount != 2 {
		t.Fatalf("expected 2 superseded facts, got %d (%#v)", supersededCount, allFacts)
	}

	for i := range allFacts {
		fact := &allFacts[i]
		if fact.Status != "superseded" {
			continue
		}
		if fact.SupersededBy == nil {
			t.Fatalf("expected superseded fact to have superseded_by, fact=%#v", fact)
		}
		next, ok := byID[*fact.SupersededBy]
		if !ok {
			t.Fatalf("expected superseded_by=%d to exist in chain", *fact.SupersededBy)
		}
		if fact.ID == next.ID {
			t.Fatalf("invalid chain: superseded fact points to itself, fact=%#v", fact)
		}
	}
}

func TestStoreCandidateFactsUpsertAndGetIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-candidate-fact-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "redis_role",
		Value:             "Redis 只做热层",
		SourceText:        "候选事实：Redis 只做热层",
		Status:            "pending",
		SourceMessageSeq:  12,
		ConfirmationCount: 0,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(redis_role) error = %v", err)
	}
	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "REDIS_ROLE",
		Value:             "Redis 只做缓存热层",
		SourceText:        "候选事实更新：Redis 只做缓存热层",
		Status:            "confirmed",
		SourceMessageSeq:  30,
		ConfirmationCount: 2,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(redis_role update) error = %v", err)
	}
	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              "active_db",
		Value:            "Postgres",
		SourceText:       "观察到主库是 Postgres",
		SourceMessageSeq: -5,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(active_db) error = %v", err)
	}

	facts, err := store.GetCandidateFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetCandidateFacts() error = %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("expected 2 candidate facts, got %d (%#v)", len(facts), facts)
	}
	if facts[0].Key != "active_db" || facts[0].Value != "Postgres" {
		t.Fatalf("expected active_db candidate fact at index 0, got %s=%s", facts[0].Key, facts[0].Value)
	}
	if facts[0].Status != "pending" {
		t.Fatalf("expected pending status for active_db, got %q", facts[0].Status)
	}
	if facts[0].SourceMessageSeq != 0 {
		t.Fatalf("expected normalized source_message_seq=0 for active_db, got %d", facts[0].SourceMessageSeq)
	}
	if facts[0].ConfirmationCount != 0 {
		t.Fatalf("expected default confirmation_count=0 for active_db, got %d", facts[0].ConfirmationCount)
	}
	if facts[1].Key != "redis_role" || facts[1].Value != "Redis 只做缓存热层" {
		t.Fatalf("expected updated redis_role candidate fact at index 1, got %s=%s", facts[1].Key, facts[1].Value)
	}
	if facts[1].Status != "confirmed" {
		t.Fatalf("expected updated status confirmed, got %q", facts[1].Status)
	}
	if facts[1].SourceMessageSeq != 30 {
		t.Fatalf("expected updated source_message_seq=30, got %d", facts[1].SourceMessageSeq)
	}
	if facts[1].ConfirmationCount != 2 {
		t.Fatalf("expected updated confirmation_count=2, got %d", facts[1].ConfirmationCount)
	}
}

func TestStoreCandidateFactsRemainIndependentFromProjectFactsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	tenantID := "tenant-memory-candidate-scope-" + suffix
	userID := "user-a"
	factKey := "runtime_policy"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               factKey,
		Value:             "候选：Postgres 作为 source of truth",
		SourceText:        "推测：Postgres 作为 source of truth",
		Status:            "pending",
		SourceMessageSeq:  8,
		ConfirmationCount: 1,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact() error = %v", err)
	}
	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              factKey,
		Value:            "已确认：Runtime 以 etcd 为 source of truth",
		SourceText:       "最终决定：Runtime 以 etcd 为 source of truth",
		SourceMessageSeq: 18,
	}); err != nil {
		t.Fatalf("UpsertProjectFact() error = %v", err)
	}

	candidateFacts, err := store.GetCandidateFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetCandidateFacts() error = %v", err)
	}
	if len(candidateFacts) != 1 {
		t.Fatalf("expected 1 candidate fact, got %d (%#v)", len(candidateFacts), candidateFacts)
	}
	if candidateFacts[0].Value != "候选：Postgres 作为 source of truth" {
		t.Fatalf("candidate fact should remain unchanged, got %q", candidateFacts[0].Value)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 1 {
		t.Fatalf("expected 1 project fact, got %d (%#v)", len(projectFacts), projectFacts)
	}
	if projectFacts[0].Value != "已确认：Runtime 以 etcd 为 source of truth" {
		t.Fatalf("project fact should remain independently stored, got %q", projectFacts[0].Value)
	}
}

func TestStoreCandidateFactsDoNotPolluteActiveFactsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-candidate-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	sessionID := "sess-candidate-" + time.Now().UTC().Format("150405.000000000")

	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionID,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "候选方案：PG is Truth"},
			{Role: "assistant", Content: "候选方案：Redis 只做热层"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(candidate-only) error = %v", err)
	}

	if err := store.RefreshSessionSummary(ctx, tenantID, userID, sessionID); err != nil {
		t.Fatalf("RefreshSessionSummary(candidate-only) error = %v", err)
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 0 {
		t.Fatalf("expected no active facts from candidate-only inputs, got %#v", activeFacts)
	}

	summary, err := store.GetSessionSummary(ctx, tenantID, userID, sessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatalf("expected session summary created after refresh")
	}
	if summary.SourceMessageSeq <= 0 {
		t.Fatalf("expected summary source_message_seq advanced, got %d", summary.SourceMessageSeq)
	}
}

func TestStoreSummaryFactDualTrackRefreshIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-dual-track-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	sessionID := "sess-dual-track-" + time.Now().UTC().Format("150405.000000000")

	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionID,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "当前目标：完成 V4 交付验证"},
			{Role: "assistant", Content: "TODO: 补充 hybrid recall 相关测试"},
			{Role: "assistant", Content: "Decision: 先保证 active facts 不被污染"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(summary-part) error = %v", err)
	}

	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              "pg_truth",
		Value:            "PG is Truth",
		SourceText:       "最终决定：PG is Truth",
		SourceMessageSeq: 10,
	}); err != nil {
		t.Fatalf("UpsertProjectFact(first) error = %v", err)
	}
	if err := store.UpsertProjectFact(ctx, ProjectFact{
		TenantID:         tenantID,
		UserID:           userID,
		Key:              "pg_truth",
		Value:            "PostgreSQL is source of truth",
		SourceText:       "最终决定：PostgreSQL is source of truth",
		SourceMessageSeq: 20,
	}); err != nil {
		t.Fatalf("UpsertProjectFact(second) error = %v", err)
	}

	if err := store.RefreshSessionSummary(ctx, tenantID, userID, sessionID); err != nil {
		t.Fatalf("RefreshSessionSummary() error = %v", err)
	}

	summary, err := store.GetSessionSummary(ctx, tenantID, userID, sessionID)
	if err != nil {
		t.Fatalf("GetSessionSummary() error = %v", err)
	}
	if summary == nil {
		t.Fatalf("expected non-nil summary")
	}
	if !containsExact(summary.OpenItems, "补充 hybrid recall 相关测试") {
		t.Fatalf("expected summary open items kept, got %#v", summary.OpenItems)
	}
	if !containsExact(summary.KeyDecisions, "先保证 active facts 不被污染") {
		t.Fatalf("expected summary key decision kept, got %#v", summary.KeyDecisions)
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 1 {
		t.Fatalf("expected single active fact, got %d (%#v)", len(activeFacts), activeFacts)
	}
	if activeFacts[0].Value != "PostgreSQL is source of truth" {
		t.Fatalf("expected latest active fact value, got %q", activeFacts[0].Value)
	}

	allFacts, err := store.getProjectFacts(ctx, tenantID, userID, true)
	if err != nil {
		t.Fatalf("getProjectFacts(includeSuperseded=true) error = %v", err)
	}
	if len(allFacts) != 2 {
		t.Fatalf("expected two facts including superseded, got %d (%#v)", len(allFacts), allFacts)
	}
}

func TestStoreCandidateFactsTableDoesNotAffectActiveFactsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-candidate-table-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if _, err := store.db.ExecContext(ctx, `
INSERT INTO candidate_facts (tenant_id, user_id, fact_key, fact_value, source_text, source_message_seq, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
`, tenantID, userID, "pg_truth", "candidate only", "候选方案：PG is Truth", 9); err != nil {
		t.Fatalf("insert candidate_facts error = %v", err)
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 0 {
		t.Fatalf("expected candidate_facts not pollute active facts, got %#v", activeFacts)
	}
}

func TestStoreHybridSemanticRecallBasicBehaviorIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-hybrid-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	sessionA := "sess-hybrid-a-" + time.Now().UTC().Format("150405.000000000")
	sessionB := "sess-hybrid-b-" + time.Now().UTC().Format("150405.000000000")

	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionA,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "postgres source truth for architecture decisions"},
			{Role: "assistant", Content: "agreed postgres is source of truth"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(sessionA) error = %v", err)
	}
	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionB,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "redis cache strategy only for hot path"},
			{Role: "assistant", Content: "redis for cache and postgres for source truth"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(sessionB) error = %v", err)
	}

	results, err := store.HybridSemanticRecall(ctx, tenantID, userID, "", "postgres source truth", 8, 8, 2)
	if err != nil {
		t.Fatalf("HybridSemanticRecall() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected non-empty hybrid recall results")
	}
	if len(results) > 2 {
		t.Fatalf("expected finalTopK=2 respected, got %d", len(results))
	}

	for _, item := range results {
		if item.AnchorSeq <= 0 {
			t.Fatalf("expected positive anchor seq, got %#v", item)
		}
		if strings.TrimSpace(item.SessionID) == "" {
			t.Fatalf("expected non-empty session id, got %#v", item)
		}
		if strings.TrimSpace(item.Snippet) == "" {
			t.Fatalf("expected non-empty snippet, got %#v", item)
		}
		if item.CombinedScore <= 0 {
			t.Fatalf("expected positive combined score, got %#v", item)
		}
	}
}

func TestStoreHybridSemanticRecallRespectsSessionScopeIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-hybrid-scope-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"
	sessionA := "sess-hybrid-scope-a-" + time.Now().UTC().Format("150405.000000000")
	sessionB := "sess-hybrid-scope-b-" + time.Now().UTC().Format("150405.000000000")

	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionA,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "postgres source truth for architecture decisions in session A"},
			{Role: "assistant", Content: "session A agrees postgres source truth"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(sessionA) error = %v", err)
	}
	if err := store.AppendFromRequest(ctx, providers.ChatCompletionRequest{
		TenantID:  tenantID,
		UserID:    userID,
		SessionID: sessionB,
		Messages: []providers.ChatMessage{
			{Role: "user", Content: "postgres source truth discussed in session B"},
			{Role: "assistant", Content: "session B confirms postgres source truth"},
		},
	}); err != nil {
		t.Fatalf("AppendFromRequest(sessionB) error = %v", err)
	}

	results, err := store.HybridSemanticRecall(ctx, tenantID, userID, sessionA, "postgres source truth", 8, 8, 5)
	if err != nil {
		t.Fatalf("HybridSemanticRecall(session filter) error = %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected non-empty results with session filter")
	}
	for _, item := range results {
		if item.SessionID != sessionA {
			t.Fatalf("expected session-scoped recall only in %q, got %q (%#v)", sessionA, item.SessionID, item)
		}
	}
}

func TestStorePromoteCandidateFactsConfirmedByUserIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-promote-confirmed-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "runtime_policy",
		Value:             "Postgres is source of truth",
		SourceText:        "用户已明确确认：Postgres is source of truth",
		Status:            "confirmed",
		SourceMessageSeq:  42,
		ConfirmationCount: 1,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(confirmed) error = %v", err)
	}

	if err := store.PromoteCandidateFacts(ctx, tenantID, userID); err != nil {
		t.Fatalf("PromoteCandidateFacts() error = %v", err)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 1 {
		t.Fatalf("expected 1 promoted active project fact, got %d (%#v)", len(projectFacts), projectFacts)
	}
	if projectFacts[0].Key != "runtime_policy" || projectFacts[0].Value != "Postgres is source of truth" {
		t.Fatalf("expected promoted runtime_policy fact, got %s=%s", projectFacts[0].Key, projectFacts[0].Value)
	}

	candidateFacts, err := store.GetCandidateFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetCandidateFacts() error = %v", err)
	}
	if len(candidateFacts) != 1 {
		t.Fatalf("expected 1 candidate fact, got %d (%#v)", len(candidateFacts), candidateFacts)
	}
	if candidateFacts[0].Status != "promoted" {
		t.Fatalf("expected candidate fact status promoted after promotion, got %q", candidateFacts[0].Status)
	}
}

func TestStorePromoteCandidateFactsByRepeatedConfirmationIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-promote-repeat-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "redis_role",
		Value:      "Redis only for cache",
		SourceText: "观察：Redis only for cache",
		Status:     "pending",
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(first) error = %v", err)
	}
	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "redis_role",
		Value:      "Redis only for cache",
		SourceText: "再次确认：Redis only for cache",
		Status:     "pending",
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(second) error = %v", err)
	}

	candidateFacts, err := store.GetCandidateFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetCandidateFacts(before promote) error = %v", err)
	}
	if len(candidateFacts) != 1 {
		t.Fatalf("expected 1 candidate fact before promotion, got %d (%#v)", len(candidateFacts), candidateFacts)
	}
	if candidateFacts[0].ConfirmationCount < 2 {
		t.Fatalf("expected confirmation_count>=2 before promotion, got %d", candidateFacts[0].ConfirmationCount)
	}

	if err := store.PromoteCandidateFacts(ctx, tenantID, userID); err != nil {
		t.Fatalf("PromoteCandidateFacts() error = %v", err)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 1 {
		t.Fatalf("expected repeated candidate fact promoted to active fact, got %d (%#v)", len(projectFacts), projectFacts)
	}
	if projectFacts[0].Key != "redis_role" || projectFacts[0].Value != "Redis only for cache" {
		t.Fatalf("expected promoted redis_role fact, got %s=%s", projectFacts[0].Key, projectFacts[0].Value)
	}
}

func TestStorePromoteCandidateFactsSkipsTentativeSignalsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-promote-tentative-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "storage_strategy",
		Value:             "Redis as source of truth",
		SourceText:        "候选方案：Redis as source of truth",
		Status:            "pending",
		ConfirmationCount: 5,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(tentative) error = %v", err)
	}

	if err := store.PromoteCandidateFacts(ctx, tenantID, userID); err != nil {
		t.Fatalf("PromoteCandidateFacts() error = %v", err)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 0 {
		t.Fatalf("expected tentative candidate fact not promoted, got %#v", projectFacts)
	}

	candidateFacts, err := store.GetCandidateFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetCandidateFacts() error = %v", err)
	}
	if len(candidateFacts) != 1 {
		t.Fatalf("expected 1 tentative candidate fact retained, got %d (%#v)", len(candidateFacts), candidateFacts)
	}
	if candidateFacts[0].Status == "promoted" {
		t.Fatalf("expected tentative candidate fact to remain non-promoted, got status=%q", candidateFacts[0].Status)
	}
}

func TestStoreCandidateFactGovernanceTransitionsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-candidate-govern-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "runtime_policy",
		Value:             "Postgres is source of truth",
		SourceText:        "候选事实：Postgres is source of truth",
		Status:            "pending",
		SourceMessageSeq:  101,
		ConfirmationCount: 1,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact() error = %v", err)
	}

	confirmed, err := store.ConfirmCandidateFact(ctx, tenantID, userID, "runtime_policy")
	if err != nil {
		t.Fatalf("ConfirmCandidateFact() error = %v", err)
	}
	if confirmed.Status != "confirmed" {
		t.Fatalf("expected confirmed status, got %q", confirmed.Status)
	}

	pendingFacts, err := store.ListCandidateFacts(ctx, tenantID, userID, "pending")
	if err != nil {
		t.Fatalf("ListCandidateFacts(pending) error = %v", err)
	}
	if len(pendingFacts) != 0 {
		t.Fatalf("expected no pending facts after confirm, got %#v", pendingFacts)
	}

	confirmedFacts, err := store.ListCandidateFacts(ctx, tenantID, userID, "confirmed")
	if err != nil {
		t.Fatalf("ListCandidateFacts(confirmed) error = %v", err)
	}
	if len(confirmedFacts) != 1 || confirmedFacts[0].Key != "runtime_policy" {
		t.Fatalf("expected one confirmed runtime_policy fact, got %#v", confirmedFacts)
	}

	promoted, err := store.PromoteCandidateFact(ctx, tenantID, userID, "runtime_policy")
	if err != nil {
		t.Fatalf("PromoteCandidateFact() error = %v", err)
	}
	if promoted.Status != "promoted" {
		t.Fatalf("expected promoted status, got %q", promoted.Status)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 1 || projectFacts[0].Key != "runtime_policy" {
		t.Fatalf("expected promoted runtime_policy active fact, got %#v", projectFacts)
	}

	promotedAgain, err := store.PromoteCandidateFact(ctx, tenantID, userID, "runtime_policy")
	if err != nil {
		t.Fatalf("PromoteCandidateFact(idempotent) error = %v", err)
	}
	if promotedAgain.Status != "promoted" {
		t.Fatalf("expected promoted status on repeated promote, got %q", promotedAgain.Status)
	}

	projectFactsAfter, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts(after idempotent promote) error = %v", err)
	}
	if len(projectFactsAfter) != 1 {
		t.Fatalf("expected exactly one active fact after repeated promote, got %#v", projectFactsAfter)
	}

	if _, err := store.RejectCandidateFact(ctx, tenantID, userID, "runtime_policy"); !errors.Is(err, ErrInvalidCandidateFactTransition) {
		t.Fatalf("expected invalid transition error for promoted->rejected, got %v", err)
	}
}

func TestStorePromoteCandidateFactsRejectedAreNeverPromotedIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-promote-rejected-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:          tenantID,
		UserID:            userID,
		Key:               "runtime_policy",
		Value:             "Postgres is source of truth",
		SourceText:        "候选事实：Postgres is source of truth",
		Status:            "pending",
		ConfirmationCount: 3,
	}); err != nil {
		t.Fatalf("UpsertCandidateFact() error = %v", err)
	}

	if _, err := store.RejectCandidateFact(ctx, tenantID, userID, "runtime_policy"); err != nil {
		t.Fatalf("RejectCandidateFact() error = %v", err)
	}
	if err := store.PromoteCandidateFacts(ctx, tenantID, userID); err != nil {
		t.Fatalf("PromoteCandidateFacts() error = %v", err)
	}

	projectFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(projectFacts) != 0 {
		t.Fatalf("expected rejected candidate never promoted, got %#v", projectFacts)
	}

	rejectedFacts, err := store.ListCandidateFacts(ctx, tenantID, userID, "rejected")
	if err != nil {
		t.Fatalf("ListCandidateFacts(rejected) error = %v", err)
	}
	if len(rejectedFacts) != 1 || rejectedFacts[0].Key != "runtime_policy" {
		t.Fatalf("expected runtime_policy still rejected, got %#v", rejectedFacts)
	}
}

func TestStoreCandidateFactRejectAndInvalidTransitionsIntegration(t *testing.T) {
	dsn := memoryTestPostgresDSN(t)
	store, err := NewStore(dsn, nil)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	ctx := context.Background()
	tenantID := "tenant-memory-candidate-reject-" + time.Now().UTC().Format("20060102150405.000000000")
	userID := "user-a"

	if err := store.UpsertCandidateFact(ctx, CandidateFact{
		TenantID:   tenantID,
		UserID:     userID,
		Key:        "cache_choice",
		Value:      "Redis as source of truth",
		SourceText: "候选方案：Redis as source of truth",
		Status:     "pending",
	}); err != nil {
		t.Fatalf("UpsertCandidateFact(cache_choice) error = %v", err)
	}

	rejected, err := store.RejectCandidateFact(ctx, tenantID, userID, "cache_choice")
	if err != nil {
		t.Fatalf("RejectCandidateFact() error = %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("expected rejected status, got %q", rejected.Status)
	}

	rejectedAgain, err := store.RejectCandidateFact(ctx, tenantID, userID, "cache_choice")
	if err != nil {
		t.Fatalf("RejectCandidateFact(idempotent) error = %v", err)
	}
	if rejectedAgain.Status != "rejected" {
		t.Fatalf("expected rejected status on repeated reject, got %q", rejectedAgain.Status)
	}

	if _, err := store.PromoteCandidateFact(ctx, tenantID, userID, "cache_choice"); !errors.Is(err, ErrInvalidCandidateFactTransition) {
		t.Fatalf("expected invalid transition error for rejected->promoted, got %v", err)
	}

	activeFacts, err := store.GetProjectFacts(ctx, tenantID, userID)
	if err != nil {
		t.Fatalf("GetProjectFacts() error = %v", err)
	}
	if len(activeFacts) != 0 {
		t.Fatalf("expected rejected candidate not affecting active facts, got %#v", activeFacts)
	}

	if _, err := store.ConfirmCandidateFact(ctx, tenantID, userID, "missing_key"); !errors.Is(err, ErrCandidateFactNotFound) {
		t.Fatalf("expected not found for missing candidate fact, got %v", err)
	}
}
