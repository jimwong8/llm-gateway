package billing

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

type InviteCode struct {
	ID             int64      `json:"id"`
	Code           string     `json:"code"`
	InviterUserID  int64      `json:"inviter_user_id"`
	InviteeUserID  *int64     `json:"invitee_user_id,omitempty"`
	TokenReward    int64      `json:"token_reward"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
}

type InviteStore interface {
	CreateInviteCode(ctx context.Context, inviterID int64, tokenReward int64) (*InviteCode, error)
	AcceptInvite(ctx context.Context, inviteeID int64, code string) error
	ListUserInvites(ctx context.Context, userID int64) ([]InviteCode, error)
}

type sqlInviteStore struct {
	db *sql.DB
}

func NewInviteStore(db *sql.DB) InviteStore {
	return &sqlInviteStore{db: db}
}

func generateInviteCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *sqlInviteStore) CreateInviteCode(ctx context.Context, inviterID int64, tokenReward int64) (*InviteCode, error) {
	code, err := generateInviteCode()
	if err != nil {
		return nil, err
	}
	var c InviteCode
	err = s.db.QueryRowContext(ctx, `
INSERT INTO invite_codes (code, inviter_user_id, token_reward, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, code, inviter_user_id, invitee_user_id, token_reward, status, expires_at, created_at, accepted_at`,
		code, inviterID, tokenReward, time.Now().Add(7*24*time.Hour),
	).Scan(&c.ID, &c.Code, &c.InviterUserID, &c.InviteeUserID, &c.TokenReward, &c.Status, &c.ExpiresAt, &c.CreatedAt, &c.AcceptedAt)
	if err != nil {
		return nil, fmt.Errorf("create invite code: %w", err)
	}
	return &c, nil
}

func (s *sqlInviteStore) AcceptInvite(ctx context.Context, inviteeID int64, code string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var c InviteCode
	err = tx.QueryRowContext(ctx, `
SELECT id, inviter_user_id, token_reward, status, expires_at
FROM invite_codes WHERE code = $1 FOR UPDATE`, code,
	).Scan(&c.ID, &c.InviterUserID, &c.TokenReward, &c.Status, &c.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid invite code")
		}
		return err
	}

	if c.Status != "pending" {
		return fmt.Errorf("invite code already used")
	}
	if c.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("invite code expired")
	}
	if c.InviterUserID == inviteeID {
		return fmt.Errorf("cannot use your own invite code")
	}

	now := time.Now()
	_, err = tx.ExecContext(ctx, `
UPDATE invite_codes SET invitee_user_id = $1, status = 'accepted', accepted_at = $2 WHERE id = $3`,
		inviteeID, now, c.ID)
	if err != nil {
		return err
	}

	if c.TokenReward > 0 {
		_, err = tx.ExecContext(ctx, `
UPDATE wallets SET balance = balance + $1, updated_at = NOW()
WHERE user_id IN ($2, $3)`, c.TokenReward, c.InviterUserID, inviteeID)
		if err != nil {
			return fmt.Errorf("reward tokens: %w", err)
		}
	}

	return tx.Commit()
}

func (s *sqlInviteStore) ListUserInvites(ctx context.Context, userID int64) ([]InviteCode, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, code, inviter_user_id, invitee_user_id, token_reward, status, expires_at, created_at, accepted_at
FROM invite_codes WHERE inviter_user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var codes []InviteCode
	for rows.Next() {
		var c InviteCode
		if err := rows.Scan(&c.ID, &c.Code, &c.InviterUserID, &c.InviteeUserID, &c.TokenReward, &c.Status, &c.ExpiresAt, &c.CreatedAt, &c.AcceptedAt); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}
