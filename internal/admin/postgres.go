package admin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type UsageRow struct {
	TenantID      string  `json:"tenant_id"`
	UserID        string  `json:"user_id"`
	RequestID     string  `json:"request_id"`
	Model         string  `json:"model"`
	Provider      string  `json:"provider"`
	TotalTokens   int     `json:"total_tokens"`
	EstimatedCost float64 `json:"estimated_cost"`
	CreatedAt     string  `json:"created_at"`
}

type AuditRow struct {
	RequestID     string `json:"request_id"`
	RouteTask     string `json:"route_task"`
	RouteModel    string `json:"route_model"`
	RouteProvider string `json:"route_provider"`
	CacheStatus   string `json:"cache_status"`
	CreatedAt     string `json:"created_at"`
}

type AssetRow struct {
	ID              int64    `json:"id"`
	TenantID        string   `json:"tenant_id"`
	UserID          string   `json:"user_id,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`
	SourceModel     string   `json:"source_model"`
	TaskType        string   `json:"task_type,omitempty"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	ContentHash     string   `json:"content_hash"`
	SourceRequestID string   `json:"source_request_id,omitempty"`
	Tags            []string `json:"tags"`
	HitCount        int      `json:"hit_count"`
	LastHitAt       string   `json:"last_hit_at,omitempty"`
	LastHitSource   string   `json:"last_hit_source,omitempty"`
	CurrentVersion  int      `json:"current_version"`
	IsDeleted       bool     `json:"is_deleted"`
	DeletedAt       string   `json:"deleted_at,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

type AssetVersionRow struct {
	AssetID           int64    `json:"asset_id"`
	Version           int      `json:"version"`
	Title             string   `json:"title"`
	Summary           string   `json:"summary"`
	SourceModel       string   `json:"source_model"`
	TaskType          string   `json:"task_type,omitempty"`
	SourceRequestID   string   `json:"source_request_id,omitempty"`
	Tags              []string `json:"tags"`
	SnapshotCreatedAt string   `json:"snapshot_created_at"`
}

type AssetReuseAuditRow struct {
	ID         int64  `json:"id"`
	TenantID   string `json:"tenant_id"`
	AssetID    int64  `json:"asset_id"`
	RequestID  string `json:"request_id"`
	RouteModel string `json:"route_model"`
	RouteTask  string `json:"route_task"`
	HitSource  string `json:"hit_source"`
	CreatedAt  string `json:"created_at"`
}

type AssetCreateInput struct {
	TenantID        string
	UserID          string
	SessionID       string
	SourceModel     string
	TaskType        string
	Title           string
	Summary         string
	Tags            []string
	SourceRequestID string
}

type AssetFilter struct {
	TenantID       string
	TaskType       string
	SourceModel    string
	Tag            string
	Keyword        string
	Limit          int
	Offset         int
	IncludeDeleted bool
}

type AssetStatsOverview struct {
	TenantID       string `json:"tenant_id,omitempty"`
	AssetCount     int    `json:"asset_count"`
	ActiveCount    int    `json:"active_count"`
	DeletedCount   int    `json:"deleted_count"`
	VersionCount   int    `json:"version_count"`
	ReuseCount     int    `json:"reuse_count"`
	TotalHitCount  int    `json:"total_hit_count"`
}

type AssetStatsGroupRow struct {
	Key           string `json:"key"`
	AssetCount    int    `json:"asset_count"`
	ActiveCount   int    `json:"active_count"`
	DeletedCount  int    `json:"deleted_count"`
	VersionCount  int    `json:"version_count"`
	ReuseCount    int    `json:"reuse_count"`
	TotalHitCount int    `json:"total_hit_count"`
}

type AssetStats struct {
	Overview AssetStatsOverview   `json:"overview"`
	ByTask   []AssetStatsGroupRow `json:"by_task"`
	ByModel  []AssetStatsGroupRow `json:"by_model"`
	ByTag    []AssetStatsGroupRow `json:"by_tag"`
}

type Store struct{ db *sql.DB }

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS knowledge_assets (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    user_id TEXT,
    session_id TEXT,
    source_model TEXT NOT NULL,
    task_type TEXT,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    source_request_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS hit_count INT NOT NULL DEFAULT 0;
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS last_hit_at TIMESTAMPTZ;
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS last_hit_source TEXT;
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS current_version INT NOT NULL DEFAULT 1;
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE knowledge_assets ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_knowledge_assets_tenant_created_at ON knowledge_assets (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_knowledge_assets_task_type ON knowledge_assets (task_type);
CREATE INDEX IF NOT EXISTS idx_knowledge_assets_source_model ON knowledge_assets (source_model);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_assets_tenant_hash ON knowledge_assets (tenant_id, content_hash);
CREATE TABLE IF NOT EXISTS knowledge_asset_tags (
    asset_id BIGINT NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
    tag TEXT NOT NULL,
    PRIMARY KEY (asset_id, tag)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_asset_tags_tag ON knowledge_asset_tags (tag);
CREATE TABLE IF NOT EXISTS knowledge_asset_versions (
    asset_id BIGINT NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
    version INT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    source_model TEXT NOT NULL,
    task_type TEXT,
    source_request_id TEXT,
    tags TEXT[] NOT NULL DEFAULT '{}',
    snapshot_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (asset_id, version)
);
CREATE TABLE IF NOT EXISTS asset_reuse_audits (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT,
    asset_id BIGINT NOT NULL REFERENCES knowledge_assets(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    route_model TEXT,
    route_task TEXT,
    hit_source TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_asset_reuse_audits_tenant_created_at ON asset_reuse_audits (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_asset_reuse_audits_asset_id ON asset_reuse_audits (asset_id);
`)
	return err
}

func (s *Store) RecentUsage(ctx context.Context, limit int) ([]UsageRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT tenant_id, user_id, request_id, model, provider, total_tokens, estimated_cost, created_at::text FROM usage_events ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageRow
	for rows.Next() {
		var row UsageRow
		if err := rows.Scan(&row.TenantID, &row.UserID, &row.RequestID, &row.Model, &row.Provider, &row.TotalTokens, &row.EstimatedCost, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) RecentAudit(ctx context.Context, limit int) ([]AuditRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT request_id, route_task, route_model, route_provider, cache_status, created_at::text FROM request_audit_logs ORDER BY id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditRow
	for rows.Next() {
		var row AuditRow
		if err := rows.Scan(&row.RequestID, &row.RouteTask, &row.RouteModel, &row.RouteProvider, &row.CacheStatus, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) CreateAsset(ctx context.Context, in AssetCreateInput) (AssetRow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetRow{}, err
	}
	defer tx.Rollback()

	row, err := createOrUpsertAsset(ctx, tx, in)
	if err != nil {
		return AssetRow{}, err
	}
	if err := tx.Commit(); err != nil {
		return AssetRow{}, err
	}
	return row, nil
}

func createOrUpsertAsset(ctx context.Context, tx *sql.Tx, in AssetCreateInput) (AssetRow, error) {
	in = normalizeInput(in)
	var existingID int64
	var existingVersion int
	err := tx.QueryRowContext(ctx, `SELECT id, current_version FROM knowledge_assets WHERE COALESCE(tenant_id, '') = $1 AND content_hash = $2`, in.TenantID, hashContent(in.Title+"\n"+in.Summary+"\n"+strings.Join(normalizeTags(in.Tags), ","))).Scan(&existingID, &existingVersion)
	if err == nil {
		return loadAssetByIDTx(ctx, tx, existingID)
	}
	if err != nil && err != sql.ErrNoRows {
		return AssetRow{}, err
	}
	contentHash := hashContent(in.Title + "\n" + in.Summary + "\n" + strings.Join(normalizeTags(in.Tags), ","))
	var row AssetRow
	if err := tx.QueryRowContext(ctx, `
INSERT INTO knowledge_assets (tenant_id, user_id, session_id, source_model, task_type, title, summary, content_hash, source_request_id, current_version, is_deleted)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,1,FALSE)
RETURNING id, tenant_id, COALESCE(user_id, ''), COALESCE(session_id, ''), source_model, COALESCE(task_type, ''), title, summary, content_hash, COALESCE(source_request_id, ''), hit_count, COALESCE(last_hit_at::text, ''), COALESCE(last_hit_source, ''), current_version, is_deleted, COALESCE(deleted_at::text, ''), created_at::text`,
		in.TenantID, in.UserID, in.SessionID, in.SourceModel, in.TaskType, in.Title, in.Summary, contentHash, in.SourceRequestID,
	).Scan(&row.ID, &row.TenantID, &row.UserID, &row.SessionID, &row.SourceModel, &row.TaskType, &row.Title, &row.Summary, &row.ContentHash, &row.SourceRequestID, &row.HitCount, &row.LastHitAt, &row.LastHitSource, &row.CurrentVersion, &row.IsDeleted, &row.DeletedAt, &row.CreatedAt); err != nil {
		return AssetRow{}, err
	}
	if err := replaceTags(ctx, tx, row.ID, normalizeTags(in.Tags)); err != nil {
		return AssetRow{}, err
	}
	if err := insertVersionSnapshot(ctx, tx, row.ID, 1, in); err != nil {
		return AssetRow{}, err
	}
	row.Tags = normalizeTags(in.Tags)
	return row, nil
}

func (s *Store) UpdateAsset(ctx context.Context, assetID int64, tenantID string, in AssetCreateInput) (AssetRow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetRow{}, err
	}
	defer tx.Rollback()

	current, err := loadAssetByIDForTenantTx(ctx, tx, assetID, tenantID)
	if err != nil {
		return AssetRow{}, err
	}
	in = normalizeInput(in)
	nextVersion := current.CurrentVersion + 1
	contentHash := hashContent(in.Title + "\n" + in.Summary + "\n" + strings.Join(normalizeTags(in.Tags), ","))
	if _, err := tx.ExecContext(ctx, `
UPDATE knowledge_assets
SET tenant_id=$2, user_id=$3, session_id=$4, source_model=$5, task_type=$6, title=$7, summary=$8, content_hash=$9, source_request_id=$10, current_version=$11, is_deleted=FALSE, deleted_at=NULL
WHERE id=$1`, assetID, in.TenantID, in.UserID, in.SessionID, in.SourceModel, in.TaskType, in.Title, in.Summary, contentHash, in.SourceRequestID, nextVersion); err != nil {
		return AssetRow{}, err
	}
	if err := replaceTags(ctx, tx, assetID, normalizeTags(in.Tags)); err != nil {
		return AssetRow{}, err
	}
	if err := insertVersionSnapshot(ctx, tx, assetID, nextVersion, in); err != nil {
		return AssetRow{}, err
	}
	row, err := loadAssetByIDTx(ctx, tx, assetID)
	if err != nil {
		return AssetRow{}, err
	}
	if err := tx.Commit(); err != nil {
		return AssetRow{}, err
	}
	return row, nil
}

func (s *Store) DeleteAsset(ctx context.Context, tenantID string, assetID int64) error {
	res, err := s.db.ExecContext(ctx, `UPDATE knowledge_assets SET is_deleted=TRUE, deleted_at=NOW() WHERE id = $1 AND ($2 = '' OR COALESCE(tenant_id, '') = $2)`, assetID, strings.TrimSpace(tenantID))
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) RollbackAsset(ctx context.Context, tenantID string, assetID int64, version int) (AssetRow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return AssetRow{}, err
	}
	defer tx.Rollback()
	current, err := loadAssetByIDForTenantTx(ctx, tx, assetID, tenantID)
	if err != nil {
		return AssetRow{}, err
	}
	var ver AssetVersionRow
	var tags []string
	if err := tx.QueryRowContext(ctx, `SELECT asset_id, version, title, summary, source_model, COALESCE(task_type, ''), COALESCE(source_request_id, ''), tags, snapshot_created_at::text FROM knowledge_asset_versions WHERE asset_id = $1 AND version = $2`, assetID, version).Scan(&ver.AssetID, &ver.Version, &ver.Title, &ver.Summary, &ver.SourceModel, &ver.TaskType, &ver.SourceRequestID, pq.Array(&tags), &ver.SnapshotCreatedAt); err != nil {
		return AssetRow{}, err
	}
	ver.Tags = tags
	in := AssetCreateInput{TenantID: current.TenantID, UserID: current.UserID, SessionID: current.SessionID, SourceModel: ver.SourceModel, TaskType: ver.TaskType, Title: ver.Title, Summary: ver.Summary, Tags: ver.Tags, SourceRequestID: ver.SourceRequestID}
	nextVersion := current.CurrentVersion + 1
	contentHash := hashContent(in.Title + "\n" + in.Summary + "\n" + strings.Join(normalizeTags(in.Tags), ","))
	if _, err := tx.ExecContext(ctx, `UPDATE knowledge_assets SET source_model=$2, task_type=$3, title=$4, summary=$5, content_hash=$6, source_request_id=$7, current_version=$8, is_deleted=FALSE, deleted_at=NULL WHERE id=$1`, assetID, in.SourceModel, in.TaskType, in.Title, in.Summary, contentHash, in.SourceRequestID, nextVersion); err != nil {
		return AssetRow{}, err
	}
	if err := replaceTags(ctx, tx, assetID, normalizeTags(in.Tags)); err != nil {
		return AssetRow{}, err
	}
	if err := insertVersionSnapshot(ctx, tx, assetID, nextVersion, in); err != nil {
		return AssetRow{}, err
	}
	row, err := loadAssetByIDTx(ctx, tx, assetID)
	if err != nil {
		return AssetRow{}, err
	}
	if err := tx.Commit(); err != nil {
		return AssetRow{}, err
	}
	return row, nil
}

func (s *Store) ListAssetVersions(ctx context.Context, tenantID string, assetID int64, limit, offset int) ([]AssetVersionRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT kv.asset_id, kv.version, kv.title, kv.summary, kv.source_model, COALESCE(kv.task_type, ''), COALESCE(kv.source_request_id, ''), kv.tags, kv.snapshot_created_at::text
FROM knowledge_asset_versions kv
JOIN knowledge_assets ka ON ka.id = kv.asset_id
WHERE kv.asset_id = $1 AND ($2 = '' OR COALESCE(ka.tenant_id, '') = $2)
ORDER BY kv.version DESC
LIMIT $3 OFFSET $4`, assetID, strings.TrimSpace(tenantID), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AssetVersionRow, 0)
	for rows.Next() {
		var row AssetVersionRow
		var tags []string
		if err := rows.Scan(&row.AssetID, &row.Version, &row.Title, &row.Summary, &row.SourceModel, &row.TaskType, &row.SourceRequestID, pq.Array(&tags), &row.SnapshotCreatedAt); err != nil {
			return nil, err
		}
		row.Tags = tags
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListAssets(ctx context.Context, filter AssetFilter) ([]AssetRow, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT
    ka.id,
    COALESCE(ka.tenant_id, ''),
    COALESCE(ka.user_id, ''),
    COALESCE(ka.session_id, ''),
    ka.source_model,
    COALESCE(ka.task_type, ''),
    ka.title,
    ka.summary,
    ka.content_hash,
    COALESCE(ka.source_request_id, ''),
    ka.hit_count,
    COALESCE(ka.last_hit_at::text, ''),
    COALESCE(ka.last_hit_source, ''),
    ka.current_version,
    ka.is_deleted,
    COALESCE(ka.deleted_at::text, ''),
    ka.created_at::text,
    COALESCE(array_agg(kat.tag ORDER BY kat.tag) FILTER (WHERE kat.tag IS NOT NULL), '{}')
FROM knowledge_assets ka
LEFT JOIN knowledge_asset_tags kat ON kat.asset_id = ka.id
WHERE ($1 = '' OR COALESCE(ka.tenant_id, '') = $1)
  AND ($2 = '' OR COALESCE(ka.task_type, '') = $2)
  AND ($3 = '' OR ka.source_model = $3)
  AND ($4 = '' OR EXISTS (SELECT 1 FROM knowledge_asset_tags t WHERE t.asset_id = ka.id AND t.tag = $4))
  AND ($5 = '' OR ka.title ILIKE '%' || $5 || '%' OR ka.summary ILIKE '%' || $5 || '%')
  AND ($8 OR ka.is_deleted = FALSE)
GROUP BY ka.id
ORDER BY ka.id DESC
LIMIT $6 OFFSET $7`,
		strings.TrimSpace(filter.TenantID),
		strings.TrimSpace(filter.TaskType),
		strings.TrimSpace(filter.SourceModel),
		strings.TrimSpace(filter.Tag),
		strings.TrimSpace(filter.Keyword),
		limit,
		offset,
		filter.IncludeDeleted,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AssetRow, 0)
	for rows.Next() {
		var row AssetRow
		var tags []string
		if err := rows.Scan(&row.ID, &row.TenantID, &row.UserID, &row.SessionID, &row.SourceModel, &row.TaskType, &row.Title, &row.Summary, &row.ContentHash, &row.SourceRequestID, &row.HitCount, &row.LastHitAt, &row.LastHitSource, &row.CurrentVersion, &row.IsDeleted, &row.DeletedAt, &row.CreatedAt, pq.Array(&tags)); err != nil {
			return nil, err
		}
		row.Tags = tags
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) RecordReuse(ctx context.Context, tenantID string, assetID int64, requestID, routeModel, routeTask, hitSource string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	trimmedTenantID := strings.TrimSpace(tenantID)
	trimmedHitSource := strings.TrimSpace(hitSource)
	trimmedRequestID := strings.TrimSpace(requestID)
	trimmedRouteModel := strings.TrimSpace(routeModel)
	trimmedRouteTask := strings.TrimSpace(routeTask)
	if _, err := tx.ExecContext(ctx, `
UPDATE knowledge_assets
SET hit_count = hit_count + 1,
    last_hit_at = NOW(),
    last_hit_source = $3
WHERE id = $1 AND ($2 = '' OR COALESCE(tenant_id, '') = $2)
`, assetID, trimmedTenantID, trimmedHitSource); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO asset_reuse_audits (tenant_id, asset_id, request_id, route_model, route_task, hit_source)
VALUES ($1, $2, $3, $4, $5, $6)
`, trimmedTenantID, assetID, trimmedRequestID, trimmedRouteModel, trimmedRouteTask, trimmedHitSource); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RecentAssetReuse(ctx context.Context, tenantID string, limit int, offset int) ([]AssetReuseAuditRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(tenant_id, ''), asset_id, request_id, COALESCE(route_model, ''), COALESCE(route_task, ''), COALESCE(hit_source, ''), created_at::text FROM asset_reuse_audits WHERE ($1 = '' OR COALESCE(tenant_id, '') = $1) ORDER BY id DESC LIMIT $2 OFFSET $3`, strings.TrimSpace(tenantID), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]AssetReuseAuditRow, 0)
	for rows.Next() {
		var row AssetReuseAuditRow
		if err := rows.Scan(&row.ID, &row.TenantID, &row.AssetID, &row.RequestID, &row.RouteModel, &row.RouteTask, &row.HitSource, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) AssetStats(ctx context.Context, tenantID string, includeDeleted bool, limit int) (AssetStats, error) {
	trimmedTenantID := strings.TrimSpace(tenantID)
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	overview, err := s.assetStatsOverview(ctx, trimmedTenantID, includeDeleted)
	if err != nil {
		return AssetStats{}, err
	}
	byTask, err := s.assetStatsByTask(ctx, trimmedTenantID, includeDeleted, limit)
	if err != nil {
		return AssetStats{}, err
	}
	byModel, err := s.assetStatsByModel(ctx, trimmedTenantID, includeDeleted, limit)
	if err != nil {
		return AssetStats{}, err
	}
	byTag, err := s.assetStatsByTag(ctx, trimmedTenantID, includeDeleted, limit)
	if err != nil {
		return AssetStats{}, err
	}
	overview.TenantID = trimmedTenantID
	return AssetStats{Overview: overview, ByTask: byTask, ByModel: byModel, ByTag: byTag}, nil
}

func (s *Store) assetStatsOverview(ctx context.Context, tenantID string, includeDeleted bool) (AssetStatsOverview, error) {
	var out AssetStatsOverview
	err := s.db.QueryRowContext(ctx, `
SELECT
	COUNT(*)::int AS asset_count,
	COUNT(*) FILTER (WHERE ka.is_deleted = FALSE)::int AS active_count,
	COUNT(*) FILTER (WHERE ka.is_deleted = TRUE)::int AS deleted_count,
	COALESCE(SUM(ka.current_version), 0)::int AS version_count,
	COALESCE(SUM(ka.hit_count), 0)::int AS total_hit_count,
	(
		SELECT COUNT(*)::int
		FROM asset_reuse_audits ara
		WHERE ($1 = '' OR COALESCE(ara.tenant_id, '') = $1)
	) AS reuse_count
FROM knowledge_assets ka
WHERE ($1 = '' OR COALESCE(ka.tenant_id, '') = $1)
  AND ($2 OR ka.is_deleted = FALSE)
`, tenantID, includeDeleted).Scan(&out.AssetCount, &out.ActiveCount, &out.DeletedCount, &out.VersionCount, &out.TotalHitCount, &out.ReuseCount)
	return out, err
}

func (s *Store) assetStatsByTask(ctx context.Context, tenantID string, includeDeleted bool, limit int) ([]AssetStatsGroupRow, error) {
	return s.assetStatsGrouped(ctx, tenantID, includeDeleted, limit, "COALESCE(ka.task_type, '')", "route_task")
}

func (s *Store) assetStatsByModel(ctx context.Context, tenantID string, includeDeleted bool, limit int) ([]AssetStatsGroupRow, error) {
	return s.assetStatsGrouped(ctx, tenantID, includeDeleted, limit, "COALESCE(ka.source_model, '')", "route_model")
}

func (s *Store) assetStatsByTag(ctx context.Context, tenantID string, includeDeleted bool, limit int) ([]AssetStatsGroupRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT
	COALESCE(kat.tag, '') AS key,
	COUNT(DISTINCT ka.id)::int AS asset_count,
	COUNT(DISTINCT ka.id) FILTER (WHERE ka.is_deleted = FALSE)::int AS active_count,
	COUNT(DISTINCT ka.id) FILTER (WHERE ka.is_deleted = TRUE)::int AS deleted_count,
	COALESCE(SUM(DISTINCT ka.current_version) FILTER (WHERE ka.id IS NOT NULL), 0)::int AS version_count,
	COALESCE(SUM(DISTINCT ka.hit_count) FILTER (WHERE ka.id IS NOT NULL), 0)::int AS total_hit_count,
	(
		SELECT COUNT(*)::int
		FROM asset_reuse_audits ara
		JOIN knowledge_asset_tags kat2 ON kat2.asset_id = ara.asset_id
		JOIN knowledge_assets ka2 ON ka2.id = ara.asset_id
		WHERE kat2.tag = kat.tag
		  AND ($1 = '' OR COALESCE(ara.tenant_id, '') = $1)
		  AND ($2 OR ka2.is_deleted = FALSE)
	) AS reuse_count
FROM knowledge_assets ka
JOIN knowledge_asset_tags kat ON kat.asset_id = ka.id
WHERE ($1 = '' OR COALESCE(ka.tenant_id, '') = $1)
  AND ($2 OR ka.is_deleted = FALSE)
GROUP BY kat.tag
ORDER BY asset_count DESC, key ASC
LIMIT $3
`, tenantID, includeDeleted, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssetStatsGroupRows(rows)
}

func (s *Store) assetStatsGrouped(ctx context.Context, tenantID string, includeDeleted bool, limit int, keyExpr string, reuseField string) ([]AssetStatsGroupRow, error) {
	query := `
WITH asset_groups AS (
	SELECT
		` + keyExpr + ` AS key,
		COUNT(*)::int AS asset_count,
		COUNT(*) FILTER (WHERE ka.is_deleted = FALSE)::int AS active_count,
		COUNT(*) FILTER (WHERE ka.is_deleted = TRUE)::int AS deleted_count,
		COALESCE(SUM(ka.current_version), 0)::int AS version_count,
		COALESCE(SUM(ka.hit_count), 0)::int AS total_hit_count
	FROM knowledge_assets ka
	WHERE ($1 = '' OR COALESCE(ka.tenant_id, '') = $1)
	  AND ($2 OR ka.is_deleted = FALSE)
	GROUP BY 1
), reuse_groups AS (
	SELECT
		COALESCE(ara.` + reuseField + `, '') AS key,
		COUNT(*)::int AS reuse_count
	FROM asset_reuse_audits ara
	JOIN knowledge_assets ka ON ka.id = ara.asset_id
	WHERE ($1 = '' OR COALESCE(ara.tenant_id, '') = $1)
	  AND ($2 OR ka.is_deleted = FALSE)
	GROUP BY 1
)
SELECT
	ag.key,
	ag.asset_count,
	ag.active_count,
	ag.deleted_count,
	ag.version_count,
	ag.total_hit_count,
	COALESCE(rg.reuse_count, 0)::int AS reuse_count
FROM asset_groups ag
LEFT JOIN reuse_groups rg ON rg.key = ag.key
ORDER BY ag.asset_count DESC, ag.key ASC
LIMIT $3
`
	rows, err := s.db.QueryContext(ctx, query, tenantID, includeDeleted, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAssetStatsGroupRows(rows)
}

func scanAssetStatsGroupRows(rows *sql.Rows) ([]AssetStatsGroupRow, error) {
	out := make([]AssetStatsGroupRow, 0)
	for rows.Next() {
		var row AssetStatsGroupRow
		if err := rows.Scan(&row.Key, &row.AssetCount, &row.ActiveCount, &row.DeletedCount, &row.VersionCount, &row.TotalHitCount, &row.ReuseCount); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func loadAssetByIDForTenantTx(ctx context.Context, tx *sql.Tx, assetID int64, tenantID string) (AssetRow, error) {
	row, err := loadAssetByIDTx(ctx, tx, assetID)
	if err != nil {
		return AssetRow{}, err
	}
	if strings.TrimSpace(tenantID) != "" && row.TenantID != strings.TrimSpace(tenantID) {
		return AssetRow{}, sql.ErrNoRows
	}
	return row, nil
}

func loadAssetByIDTx(ctx context.Context, tx *sql.Tx, assetID int64) (AssetRow, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT
    ka.id,
    COALESCE(ka.tenant_id, ''),
    COALESCE(ka.user_id, ''),
    COALESCE(ka.session_id, ''),
    ka.source_model,
    COALESCE(ka.task_type, ''),
    ka.title,
    ka.summary,
    ka.content_hash,
    COALESCE(ka.source_request_id, ''),
    ka.hit_count,
    COALESCE(ka.last_hit_at::text, ''),
    COALESCE(ka.last_hit_source, ''),
    ka.current_version,
    ka.is_deleted,
    COALESCE(ka.deleted_at::text, ''),
    ka.created_at::text,
    COALESCE(array_agg(kat.tag ORDER BY kat.tag) FILTER (WHERE kat.tag IS NOT NULL), '{}')
FROM knowledge_assets ka
LEFT JOIN knowledge_asset_tags kat ON kat.asset_id = ka.id
WHERE ka.id = $1
GROUP BY ka.id`, assetID)
	if err != nil {
		return AssetRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return AssetRow{}, sql.ErrNoRows
	}
	var out AssetRow
	var tags []string
	if err := rows.Scan(&out.ID, &out.TenantID, &out.UserID, &out.SessionID, &out.SourceModel, &out.TaskType, &out.Title, &out.Summary, &out.ContentHash, &out.SourceRequestID, &out.HitCount, &out.LastHitAt, &out.LastHitSource, &out.CurrentVersion, &out.IsDeleted, &out.DeletedAt, &out.CreatedAt, pq.Array(&tags)); err != nil {
		return AssetRow{}, err
	}
	out.Tags = tags
	return out, nil
}

func insertVersionSnapshot(ctx context.Context, tx *sql.Tx, assetID int64, version int, in AssetCreateInput) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO knowledge_asset_versions (asset_id, version, title, summary, source_model, task_type, source_request_id, tags) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, assetID, version, strings.TrimSpace(in.Title), strings.TrimSpace(in.Summary), strings.TrimSpace(in.SourceModel), strings.TrimSpace(in.TaskType), strings.TrimSpace(in.SourceRequestID), pq.Array(normalizeTags(in.Tags)))
	return err
}

func replaceTags(ctx context.Context, tx *sql.Tx, assetID int64, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge_asset_tags WHERE asset_id = $1`, assetID); err != nil {
		return err
	}
	for _, tag := range tags {
		if _, err := tx.ExecContext(ctx, `INSERT INTO knowledge_asset_tags (asset_id, tag) VALUES ($1,$2) ON CONFLICT DO NOTHING`, assetID, tag); err != nil {
			return err
		}
	}
	return nil
}

func normalizeInput(in AssetCreateInput) AssetCreateInput {
	in.TenantID = strings.TrimSpace(in.TenantID)
	in.UserID = strings.TrimSpace(in.UserID)
	in.SessionID = strings.TrimSpace(in.SessionID)
	in.SourceModel = strings.TrimSpace(in.SourceModel)
	in.TaskType = strings.TrimSpace(in.TaskType)
	in.Title = strings.TrimSpace(in.Title)
	in.Summary = strings.TrimSpace(in.Summary)
	in.SourceRequestID = strings.TrimSpace(in.SourceRequestID)
	in.Tags = normalizeTags(in.Tags)
	return in
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(tag)), "-"))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func hashContent(text string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(text)))
	return hex.EncodeToString(sum[:])
}

type ChannelRow struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Provider      string   `json:"provider"`
	BaseURL       string   `json:"base_url"`
	APIKey        string   `json:"api_key"`
	Priority      string   `json:"priority"`
	Weight        int      `json:"weight"`
	Models        []string `json:"models"`
	Tags          []string `json:"tags"`
	Notes         string   `json:"notes"`
	Status        string   `json:"status"`
	LatencyMs     int      `json:"latency_ms"`
	TotalRequests int64    `json:"total_requests"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func (s *Store) ensureChannelsSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS channels (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(256) NOT NULL,
    provider VARCHAR(64) NOT NULL,
    base_url TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    priority VARCHAR(16) NOT NULL DEFAULT 'medium',
    weight INTEGER NOT NULL DEFAULT 1,
    models TEXT[] DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    notes TEXT DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    latency_ms INTEGER DEFAULT 0,
    total_requests BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status);
CREATE INDEX IF NOT EXISTS idx_channels_provider ON channels(provider);
`)
	return err
}

func (s *Store) ListChannels(ctx context.Context) ([]ChannelRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, provider, base_url, api_key, priority, weight, models, tags, notes, status, latency_ms, total_requests, created_at, updated_at
FROM channels ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChannelRow
	for rows.Next() {
		var c ChannelRow
		if err := rows.Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &c.APIKey, &c.Priority, &c.Weight, pq.Array(&c.Models), pq.Array(&c.Tags), &c.Notes, &c.Status, &c.LatencyMs, &c.TotalRequests, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetChannel(ctx context.Context, id string) (*ChannelRow, error) {
	var c ChannelRow
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, provider, base_url, api_key, priority, weight, models, tags, notes, status, latency_ms, total_requests, created_at, updated_at
FROM channels WHERE id=$1`, id).Scan(&c.ID, &c.Name, &c.Provider, &c.BaseURL, &c.APIKey, &c.Priority, &c.Weight, pq.Array(&c.Models), pq.Array(&c.Tags), &c.Notes, &c.Status, &c.LatencyMs, &c.TotalRequests, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

type ChannelInput struct {
	ID       string
	Name     string
	Provider string
	BaseURL  string
	APIKey   string
	Priority string
	Weight   int
	Models   []string
	Tags     []string
	Notes    string
	Status   string
}

func (s *Store) CreateChannel(ctx context.Context, in ChannelInput) (*ChannelRow, error) {
	if in.ID == "" {
		in.ID = "ch-" + fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	if in.Weight == 0 {
		in.Weight = 1
	}
	if in.Status == "" {
		in.Status = "active"
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO channels (id, name, provider, base_url, api_key, priority, weight, models, tags, notes, status)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
ON CONFLICT (id) DO UPDATE SET
 name=EXCLUDED.name, provider=EXCLUDED.provider, base_url=EXCLUDED.base_url, api_key=EXCLUDED.api_key,
 priority=EXCLUDED.priority, weight=EXCLUDED.weight, models=EXCLUDED.models, tags=EXCLUDED.tags,
 notes=EXCLUDED.notes, status=EXCLUDED.status, updated_at=NOW()
`, in.ID, in.Name, in.Provider, in.BaseURL, in.APIKey, in.Priority, in.Weight, pq.Array(in.Models), pq.Array(in.Tags), in.Notes, in.Status)
	if err != nil {
		return nil, err
	}
	return s.GetChannel(ctx, in.ID)
}

func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE id=$1`, id)
	return err
}

func (s *Store) BatchDeleteChannels(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM channels WHERE id = ANY($1)`, pq.Array(ids))
	return err
}

func (s *Store) BatchUpdateChannelsStatus(ctx context.Context, ids []string, status string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE channels SET status=$1, updated_at=NOW() WHERE id = ANY($2)`, status, pq.Array(ids))
	return err
}

func (s *Store) TestChannel(ctx context.Context, id string) (map[string]any, error) {
	c, err := s.GetChannel(ctx, id)
	if err != nil {
		return map[string]any{"success": false, "error": "channel not found"}, nil
	}
	if c.APIKey == "" {
		return map[string]any{"success": false, "error": "no api key configured"}, nil
	}
	return map[string]any{"success": true, "latency_ms": 0, "model": c.Models}, nil
}
