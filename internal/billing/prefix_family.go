package billing

import "context"

// PrefixFamilyStatRow aggregates usage_events by prompt prefix family so the
// dashboard can show which prefix families are reusable (>=2 requests share a
// provider prompt cache) and how many cached tokens they saved.
type PrefixFamilyStatRow struct {
	PrefixFamily string  `json:"prefix_family"`
	Requests     int64   `json:"requests"`
	CachedTokens int64   `json:"cached_tokens"`
	Reusable     bool    `json:"reusable"`
	HitRate      float64 `json:"hit_rate"`
}

// PrefixFamilyStats returns per-prefix-family aggregates for the filter window.
// Reusable is true when the family served more than one request, meaning a
// provider prefix cache can serve subsequent requests.
func (s *Store) PrefixFamilyStats(ctx context.Context, filter QueryFilter) ([]PrefixFamilyStatRow, error) {
	query, args := buildWhere(`
SELECT
	COALESCE(prefix_family, '') AS prefix_family,
	COUNT(*) AS requests,
	COALESCE(SUM(cached_tokens), 0) AS cached_tokens,
	COALESCE(AVG(CASE WHEN cache_status <> 'MISS' THEN 1.0 ELSE 0.0 END), 0) AS hit_rate
FROM usage_events
`, filter)
	query += ` GROUP BY prefix_family HAVING COALESCE(prefix_family, '') <> '' ORDER BY requests DESC, prefix_family ASC`
	if filter.Limit > 0 {
		query += " LIMIT $0"
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PrefixFamilyStatRow{}
	for rows.Next() {
		var row PrefixFamilyStatRow
		if err := rows.Scan(&row.PrefixFamily, &row.Requests, &row.CachedTokens, &row.HitRate); err != nil {
			return nil, err
		}
		row.Reusable = row.Requests > 1
		out = append(out, row)
	}
	return out, rows.Err()
}
