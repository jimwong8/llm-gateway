package longcontext

import (
	"context"
	"fmt"
)

// MaxConcurrencyPerTenant limits how many non-terminal tasks a single tenant
// may have at once. This prevents a single tenant from flooding the worker
// pool or racking up unbounded cost.
const MaxConcurrencyPerTenant = 3

// CountRunningTasksByTenant returns the number of tasks for a tenant that are
// not in a terminal state.
func (r *PostgresRepository) CountRunningTasksByTenant(ctx context.Context, tenantID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM long_context_tasks WHERE tenant_id=$1 AND status NOT IN ($2,$3,$4)`, tenantID, StatusSucceeded, StatusFailed, StatusCanceled).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count running tasks: %w", err)
	}
	return n, nil
}

// EnforceTenantConcurrency checks whether creating a new task for tenant would
// exceed the per-tenant concurrency limit.
func EnforceTenantConcurrency(ctx context.Context, r *PostgresRepository, tenantID string) error {
	n, err := r.CountRunningTasksByTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if n >= MaxConcurrencyPerTenant {
		return fmt.Errorf("tenant %s has reached the maximum of %d concurrent long-context tasks", tenantID, MaxConcurrencyPerTenant)
	}
	return nil
}
