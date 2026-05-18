package billing

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type LedgerFilter struct {
	UserID string
	Type   string
	From   time.Time
	To     time.Time
	Limit  int
	Offset int
}

func (s *Store) ListLedger(ctx context.Context, filter LedgerFilter) ([]LedgerEntry, error) {
	clauses := []string{"1=1"}
	args := []any{}
	paramIdx := 0

	if strings.TrimSpace(filter.UserID) != "" {
		paramIdx++
		args = append(args, filter.UserID)
		clauses = append(clauses, fmt.Sprintf("user_id = $%d", paramIdx))
	}
	if strings.TrimSpace(filter.Type) != "" {
		paramIdx++
		args = append(args, filter.Type)
		clauses = append(clauses, fmt.Sprintf("type = $%d", paramIdx))
	}
	if !filter.From.IsZero() {
		paramIdx++
		args = append(args, filter.From)
		clauses = append(clauses, fmt.Sprintf("created_at >= $%d", paramIdx))
	}
	if !filter.To.IsZero() {
		paramIdx++
		args = append(args, filter.To)
		clauses = append(clauses, fmt.Sprintf("created_at <= $%d", paramIdx))
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
SELECT id, user_id, type, amount, balance_after, reference_id, description, created_at
FROM ledger
WHERE %s
ORDER BY created_at DESC, id DESC
LIMIT $%d OFFSET $%d`,
		strings.Join(clauses, " AND "), paramIdx+1, paramIdx+2)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LedgerEntry
	for rows.Next() {
		var e LedgerEntry
		var desc sql.NullString
		if err := rows.Scan(&e.ID, &e.UserID, &e.Type, &e.Amount, &e.BalanceAfter, &e.ReferenceID, &desc, &e.CreatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			e.Description = desc.String
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
