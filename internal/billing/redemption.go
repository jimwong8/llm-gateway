package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type RedemptionCode struct {
	ID           int64      `json:"id"`
	Code         string     `json:"code"`
	CodeType     string     `json:"code_type"`
	TokenAmount  int64      `json:"token_amount"`
	PlanID       *int64     `json:"plan_id,omitempty"`
	ValidFrom    time.Time  `json:"valid_from"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	MaxUses      int        `json:"max_uses"`
	CurrentUses  int        `json:"current_uses"`
	IsActive     bool       `json:"is_active"`
}

type RedemptionStore interface {
	CreateCode(ctx context.Context, codeType string, tokenAmount int64, planID *int64, validUntil *time.Time, maxUses int) (*RedemptionCode, error)
	RedeemCode(ctx context.Context, userID int64, code string) (tokenAmount int64, err error)
}

type sqlRedemptionStore struct {
	db *sql.DB
}

func NewRedemptionStore(db *sql.DB) RedemptionStore {
	return &sqlRedemptionStore{db: db}
}

func generateCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s-%s", s[:4], s[4:8], s[8:12]), nil
}

func (s *sqlRedemptionStore) CreateCode(ctx context.Context, codeType string, tokenAmount int64, planID *int64, validUntil *time.Time, maxUses int) (*RedemptionCode, error) {
	code, err := generateCode()
	if err != nil {
		return nil, err
	}
	var c RedemptionCode
	err = s.db.QueryRowContext(ctx, `
INSERT INTO redemption_codes (code, code_type, token_amount, plan_id, valid_until, max_uses)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, code, code_type, token_amount, plan_id, valid_from, valid_until, max_uses, current_uses, is_active`,
		code, codeType, tokenAmount, planID, validUntil, maxUses,
	).Scan(&c.ID, &c.Code, &c.CodeType, &c.TokenAmount, &c.PlanID, &c.ValidFrom, &c.ValidUntil, &c.MaxUses, &c.CurrentUses, &c.IsActive)
	if err != nil {
		return nil, fmt.Errorf("create redemption code: %w", err)
	}
	return &c, nil
}

func (s *sqlRedemptionStore) RedeemCode(ctx context.Context, userID int64, code string) (int64, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return 0, fmt.Errorf("code is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var codeID, tokenAmount, maxUses, currentUses int64
	var isActive bool
	var validUntil *time.Time
	err = tx.QueryRowContext(ctx, `
SELECT id, token_amount, max_uses, current_uses, is_active, valid_until
FROM redemption_codes WHERE code = $1 FOR UPDATE`, code,
	).Scan(&codeID, &tokenAmount, &maxUses, &currentUses, &isActive, &validUntil)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("invalid code")
		}
		return 0, err
	}

	if !isActive {
		return 0, fmt.Errorf("code is not active")
	}
	if validUntil != nil && validUntil.Before(time.Now()) {
		return 0, fmt.Errorf("code has expired")
	}
	if currentUses >= maxUses {
		return 0, fmt.Errorf("code has been fully used")
	}

	_, err = tx.ExecContext(ctx, `UPDATE redemption_codes SET current_uses = current_uses + 1 WHERE id = $1`, codeID)
	if err != nil {
		return 0, err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO redemption_history (user_id, code_id) VALUES ($1, $2)`, userID, codeID)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return tokenAmount, nil
}
