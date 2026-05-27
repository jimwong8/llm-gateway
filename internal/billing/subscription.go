package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Plan struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	PriceCents        int64     `json:"price_cents"`
	Interval          string    `json:"interval"`
	TokenQuota        int64     `json:"token_quota"`
	RateLimitPerMinute int      `json:"rate_limit_per_minute"`
	Features          string    `json:"features"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type UserSubscription struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	PlanID      int64      `json:"plan_id"`
	PlanName    string     `json:"plan_name,omitempty"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

type SubscriptionStore interface {
	CreatePlan(ctx context.Context, name, description string, priceCents int64, interval string, tokenQuota int64, rateLimit int) (*Plan, error)
	ListActivePlans(ctx context.Context) ([]Plan, error)
	GetPlanByID(ctx context.Context, id int64) (*Plan, error)
	SubscribeUser(ctx context.Context, userID, planID int64, duration time.Duration) (*UserSubscription, error)
	GetUserSubscription(ctx context.Context, userID int64) (*UserSubscription, error)
	CancelSubscription(ctx context.Context, userID int64) error
}

type sqlSubscriptionStore struct {
	db *sql.DB
}

func NewSubscriptionStore(db *sql.DB) SubscriptionStore {
	return &sqlSubscriptionStore{db: db}
}

func (s *sqlSubscriptionStore) CreatePlan(ctx context.Context, name, description string, priceCents int64, interval string, tokenQuota int64, rateLimit int) (*Plan, error) {
	var p Plan
	err := s.db.QueryRowContext(ctx, `
INSERT INTO subscription_plans (name, description, price_cents, interval, token_quota, rate_limit_per_minute)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, name, description, price_cents, interval, token_quota, rate_limit_per_minute, features, is_active, created_at, updated_at`,
		name, description, priceCents, interval, tokenQuota, rateLimit,
	).Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents, &p.Interval, &p.TokenQuota, &p.RateLimitPerMinute, &p.Features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create plan: %w", err)
	}
	return &p, nil
}

func (s *sqlSubscriptionStore) ListActivePlans(ctx context.Context) ([]Plan, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, description, price_cents, interval, token_quota, rate_limit_per_minute, features, is_active, created_at, updated_at
FROM subscription_plans WHERE is_active = TRUE ORDER BY price_cents ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents, &p.Interval, &p.TokenQuota, &p.RateLimitPerMinute, &p.Features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *sqlSubscriptionStore) GetPlanByID(ctx context.Context, id int64) (*Plan, error) {
	var p Plan
	err := s.db.QueryRowContext(ctx, `
SELECT id, name, description, price_cents, interval, token_quota, rate_limit_per_minute, features, is_active, created_at, updated_at
FROM subscription_plans WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.PriceCents, &p.Interval, &p.TokenQuota, &p.RateLimitPerMinute, &p.Features, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *sqlSubscriptionStore) SubscribeUser(ctx context.Context, userID, planID int64, duration time.Duration) (*UserSubscription, error) {
	plan, err := s.GetPlanByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	expiresAt := time.Now().Add(duration)
	var sub UserSubscription
	err = s.db.QueryRowContext(ctx, `
INSERT INTO user_subscriptions (user_id, plan_id, status, expires_at)
VALUES ($1, $2, 'active', $3)
ON CONFLICT (user_id) WHERE status = 'active' DO UPDATE SET plan_id = $2, status = 'active', expires_at = $3, cancelled_at = NULL, updated_at = NOW()
RETURNING id, user_id, plan_id, status, started_at, expires_at`,
		userID, planID, expiresAt,
	).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.Status, &sub.StartedAt, &sub.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("subscribe user: %w", err)
	}
	sub.PlanName = plan.Name
	return &sub, nil
}

func (s *sqlSubscriptionStore) GetUserSubscription(ctx context.Context, userID int64) (*UserSubscription, error) {
	var sub UserSubscription
	err := s.db.QueryRowContext(ctx, `
SELECT s.id, s.user_id, s.plan_id, p.name, s.status, s.started_at, s.expires_at, s.cancelled_at
FROM user_subscriptions s
JOIN subscription_plans p ON p.id = s.plan_id
WHERE s.user_id = $1 AND s.status = 'active' AND s.expires_at > NOW()
ORDER BY s.created_at DESC LIMIT 1`, userID,
	).Scan(&sub.ID, &sub.UserID, &sub.PlanID, &sub.PlanName, &sub.Status, &sub.StartedAt, &sub.ExpiresAt, &sub.CancelledAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (s *sqlSubscriptionStore) CancelSubscription(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE user_subscriptions SET status = 'cancelled', cancelled_at = NOW(), updated_at = NOW()
WHERE user_id = $1 AND status = 'active'`, userID)
	return err
}
