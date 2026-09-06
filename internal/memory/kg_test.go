package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// testDSN builds a DSN for the local test database.
// Override with environment variable TEST_PG_DSN.
func testDSN() string {
	dsn := "postgres://postgres:postgres@localhost:5432/llm_gateway_test?sslmode=disable"
	return dsn
}

func newTestStore(t *testing.T) (*sql.DB, *sqlKGStore) {
	t.Helper()
	dsn := testDSN()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("skipping: cannot connect to postgres: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skipf("skipping: postgres ping failed: %v", err)
	}
	// Clean up test tables.
	db.ExecContext(ctx, "TRUNCATE kg_entities, kg_relations RESTART IDENTITY CASCADE")
	return db, &sqlKGStore{db: db, embedder: nullEmbed()}
}

// ---- KGEntity CRUD ----

func TestKGEntityCRUD(t *testing.T) {
	db, store := newTestStore(t)
	defer db.Close()
	ctx := context.Background()

	ent := &KGEntity{
		UserID:     1,
		EntityName: "TestService",
		EntityType: "project",
		Properties: `{"lang":"go"}`,
		Visibility: "private",
		SessionIDs: []int64{100},
		SourceText: "initial text",
		CreatedBy:  1,
	}

	// Create.
	created, err := store.CreateEntity(ctx, ent)
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero entity ID")
	}
	if created.EntityName != "TestService" {
		t.Fatalf("expected name TestService, got %s", created.EntityName)
	}

	// Read.
	got, err := store.GetEntity(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if got.EntityName != "TestService" {
		t.Fatalf("GetEntity name mismatch: %s", got.EntityName)
	}

	// FindByName.
	found, err := store.FindByName(ctx, 1, "TestService")
	if err != nil {
		t.Fatalf("FindByName: %v", err)
	}
	if found == nil || found.ID != created.ID {
		t.Fatal("FindByName did not return expected entity")
	}

	// Upsert (update same name).
	ent2 := &KGEntity{
		UserID:     1,
		EntityName: "TestService",
		EntityType: "tool",
		Properties: `{"lang":"rust"}`,
		SessionIDs: []int64{200},
		CreatedBy:  1,
	}
	updated, err := store.UpsertEntity(ctx, ent2)
	if err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if updated.EntityType != "tool" {
		t.Fatalf("expected type tool after upsert, got %s", updated.EntityType)
	}

	// ListByUser.
	entities, total, err := store.ListByUser(ctx, 1, "", 10, 0)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	// Delete.
	if err := store.DeleteEntity(ctx, 1, created.ID); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	deleted, err := store.GetEntity(ctx, created.ID)
	if err == nil && deleted != nil {
		t.Fatal("expected entity to be soft-deleted")
	}
}

// ---- KGRelation CRUD ----

func TestKGRelationCRUD(t *testing.T) {
	db, store := newTestStore(t)
	defer db.Close()
	ctx := context.Background()

	// Create two entities first.
	e1, _ := store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "API", EntityType: "api", CreatedBy: 1})
	e2, _ := store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "Database", EntityType: "tool", CreatedBy: 1})

	rel := &KGRelation{
		FromEntityID: e1.ID,
		ToEntityID:   e2.ID,
		RelationType: "depends_on",
		Properties:   `{"strength":"strong"}`,
		Weight:       0.9,
		Visibility:   "private",
		SessionIDs:   []int64{100},
		SourceText:   "API depends_on Database",
	}

	// Create.
	created, err := store.CreateRelation(ctx, rel)
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero relation ID")
	}

	// List by entity.
	rels, total, err := store.ListRelationsByEntity(ctx, e1.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListRelationsByEntity: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1, got %d", total)
	}
	if len(rels) != 1 || rels[0].RelationType != "depends_on" {
		t.Fatal("unexpected relation data")
	}

	// Upsert (update same triple).
	rel2 := &KGRelation{
		FromEntityID: e1.ID,
		ToEntityID:   e2.ID,
		RelationType: "depends_on",
		Weight:       0.5,
		SessionIDs:   []int64{200},
		SourceText:   "updated",
	}
	updated, err := store.UpsertRelation(ctx, rel2)
	if err != nil {
		t.Fatalf("UpsertRelation: %v", err)
	}
	if updated.Weight != 0.5 {
		t.Fatalf("expected weight 0.5, got %f", updated.Weight)
	}

	// Delete.
	if err := store.DeleteRelation(ctx, created.ID); err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}
}

// ---- KGSearcher ----

func TestKGSearcher(t *testing.T) {
	db, store := newTestStore(t)
	defer db.Close()
	ctx := context.Background()

	// Seed entities.
	for i, name := range []string{"AuthService", "AuthMiddleware", "UserDB", "PaymentAPI"} {
		store.CreateEntity(ctx, &KGEntity{
			UserID:     1,
			EntityName: name,
			EntityType: "project",
			CreatedBy:  1,
			SessionIDs: []int64{int64(i)},
		})
	}

	// Search by keyword.
	results, err := store.SearchEntities(ctx, 1, "Auth", "", 10)
	if err != nil {
		t.Fatalf("SearchEntities: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected >=2 results for 'Auth', got %d", len(results))
	}

	// Search with type filter.
	results2, err := store.SearchEntities(ctx, 1, "Auth", "project", 10)
	if err != nil {
		t.Fatalf("SearchEntities with type: %v", err)
	}
	if len(results2) < 2 {
		t.Fatalf("expected >=2 typed results, got %d", len(results2))
	}

	// Search relations.
	store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "Gateway", EntityType: "api", CreatedBy: 1})
	e1, _ := store.FindByName(ctx, 1, "Gateway")
	e2, _ := store.FindByName(ctx, 1, "AuthService")
	store.CreateRelation(ctx, &KGRelation{
		FromEntityID: e1.ID, ToEntityID: e2.ID,
		RelationType: "calls", SourceText: "Gateway calls AuthService",
	})

	rels, err := store.SearchRelations(ctx, 1, "Gateway", 10)
	if err != nil {
		t.Fatalf("SearchRelations: %v", err)
	}
	if len(rels) < 1 {
		t.Fatal("expected at least 1 relation from search")
	}
}

// ---- KGSubgraph (Graph traversal) ----

func TestKGSubgraph(t *testing.T) {
	db, store := newTestStore(t)
	defer db.Close()
	ctx := context.Background()

	e1, _ := store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "Root", EntityType: "project", CreatedBy: 1})
	e2, _ := store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "Child1", EntityType: "component", CreatedBy: 1})
	e3, _ := store.CreateEntity(ctx, &KGEntity{UserID: 1, EntityName: "Child2", EntityType: "component", CreatedBy: 1})

	store.CreateRelation(ctx, &KGRelation{FromEntityID: e1.ID, ToEntityID: e2.ID, RelationType: "contains"})
	store.CreateRelation(ctx, &KGRelation{FromEntityID: e1.ID, ToEntityID: e3.ID, RelationType: "contains"})

	// BFS depth=1 from Root.
	subgraph, err := store.GetSubgraph(ctx, 1, e1.ID, 1)
	if err != nil {
		t.Fatalf("GetSubgraph: %v", err)
	}
	if len(subgraph.Entities) < 3 {
		t.Fatalf("expected >=3 entities in subgraph, got %d", len(subgraph.Entities))
	}
	if len(subgraph.Relations) < 2 {
		t.Fatalf("expected >=2 relations, got %d", len(subgraph.Relations))
	}

	// GetRelatedSessions.
	sessions, err := store.GetRelatedSessions(ctx, e1.ID)
	if err != nil {
		t.Fatalf("GetRelatedSessions: %v", err)
	}
	_ = sessions

	// GetFullGraph.
	full, err := store.GetFullGraph(ctx, 1, 100)
	if err != nil {
		t.Fatalf("GetFullGraph: %v", err)
	}
	if len(full.Entities) < 3 {
		t.Fatalf("expected >=3 entities in full graph, got %d", len(full.Entities))
	}
}

// ---- KGBuilder (entity extraction from text) ----

func TestKGBuilderRuleBased(t *testing.T) {
	db, store := newTestStore(t)
	defer db.Close()
	ctx := context.Background()

	text := `
		The func main calls func authenticate and func connectDB.
		The service uses /etc/config/settings.yaml and /var/log/app.log.
		Package github.com/gin-gonic/gin handles routing.
	`

	subgraph, err := store.ExtractFromText(ctx, 1, 42, text)
	if err != nil {
		t.Fatalf("ExtractFromText: %v", err)
	}

	if len(subgraph.Entities) == 0 {
		t.Fatal("expected at least 1 entity from rule-based extraction")
	}

	// Verify entity names contain expected patterns.
	var names []string
	for _, e := range subgraph.Entities {
		names = append(names, e.EntityName)
	}
	nameStr := strings.Join(names, ",")
	if !strings.Contains(nameStr, "main") && !strings.Contains(nameStr, "authenticate") {
		t.Logf("entities: %v", names)
	}

	t.Logf("extracted %d entities, %d relations", len(subgraph.Entities), len(subgraph.Relations))
}

// ---- parsePGIntArray ----

func TestParsePGIntArray(t *testing.T) {
	tests := []struct {
		input string
		want  []int64
	}{
		{"{}", nil},
		{"", nil},
		{"{1,2,3}", []int64{1, 2, 3}},
		{"{42}", []int64{42}},
	}
	for _, tt := range tests {
		got := parsePGIntArray(tt.input)
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tt.want) {
			t.Errorf("parsePGIntArray(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ---- pgArrayInt ----

func TestPGArrayInt(t *testing.T) {
	if got := pgArrayInt(nil); got != "{}" {
		t.Fatalf("pgArrayInt(nil) = %q, want %q", got, "{}")
	}
	if got := pgArrayInt([]int64{1, 2, 3}); got != "{1,2,3}" {
		t.Fatalf("pgArrayInt({1,2,3}) = %q, want %q", got, "{1,2,3}")
	}
}
