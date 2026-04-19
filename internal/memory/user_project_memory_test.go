package memory

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
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
