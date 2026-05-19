package billing

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LedgerEntry struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	Type         string    `json:"type"`
	Amount       float64   `json:"amount"`
	BalanceAfter float64   `json:"balance_after"`
	ReferenceID  string    `json:"reference_id"`
	Description  string    `json:"description"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *Store) EnsureWallet(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO wallets (user_id, balance, currency, updated_at)
VALUES ($1, 0, 'USD', NOW())
ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

func (s *Store) GetBalance(ctx context.Context, userID string) (float64, error) {
	var balance sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `
SELECT balance FROM wallets WHERE user_id = $1`, userID).Scan(&balance)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !balance.Valid {
		return 0, nil
	}
	return balance.Float64, nil
}

func (s *Store) Credit(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	return s.ledgerAppend(ctx, userID, amount, txType, description, referenceID)
}

func (s *Store) Debit(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	return s.ledgerAppend(ctx, userID, -amount, txType, description, referenceID)
}

func (s *Store) ledgerAppend(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	if amount >= 0 {
		return s.creditInternal(ctx, userID, amount, txType, description, referenceID)
	}
	return s.debitInternal(ctx, userID, -amount, txType, description, referenceID)
}

func (s *Store) creditInternal(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if referenceID != "" {
		var existing int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger WHERE reference_id = $1`, referenceID).Scan(&existing)
		if err != nil {
			return nil, err
		}
		if existing > 0 {
			var entry LedgerEntry
			err := tx.QueryRowContext(ctx, `
SELECT id, user_id, type, amount, balance_after, reference_id, description, created_at
FROM ledger WHERE reference_id = $1`, referenceID).Scan(
				&entry.ID, &entry.UserID, &entry.Type, &entry.Amount,
				&entry.BalanceAfter, &entry.ReferenceID, &entry.Description, &entry.CreatedAt)
			if err != nil {
				return nil, err
			}
			return &entry, nil
		}
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO wallets (user_id, balance, currency, updated_at)
VALUES ($1, 0, 'USD', NOW())
ON CONFLICT (user_id) DO NOTHING`, userID)
	if err != nil {
		return nil, err
	}

	var newBalance float64
	err = tx.QueryRowContext(ctx, `
UPDATE wallets SET balance = balance + $1, updated_at = NOW()
WHERE user_id = $2
RETURNING balance`, amount, userID).Scan(&newBalance)
	if err != nil {
		return nil, err
	}

	var entry LedgerEntry
	err = tx.QueryRowContext(ctx, `
INSERT INTO ledger (user_id, type, amount, balance_after, reference_id, description)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, type, amount, balance_after, reference_id, description, created_at`,
		userID, txType, amount, newBalance, referenceID, description,
	).Scan(&entry.ID, &entry.UserID, &entry.Type, &entry.Amount,
		&entry.BalanceAfter, &entry.ReferenceID, &entry.Description, &entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *Store) debitInternal(ctx context.Context, userID string, absAmount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if referenceID != "" {
		var existing int
		err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ledger WHERE reference_id = $1`, referenceID).Scan(&existing)
		if err != nil {
			return nil, err
		}
		if existing > 0 {
			var entry LedgerEntry
			err := tx.QueryRowContext(ctx, `
SELECT id, user_id, type, amount, balance_after, reference_id, description, created_at
FROM ledger WHERE reference_id = $1`, referenceID).Scan(
				&entry.ID, &entry.UserID, &entry.Type, &entry.Amount,
				&entry.BalanceAfter, &entry.ReferenceID, &entry.Description, &entry.CreatedAt)
			if err != nil {
				return nil, err
			}
			return &entry, nil
		}
	}

	var currentBalance sql.NullFloat64
	err = tx.QueryRowContext(ctx, `SELECT balance FROM wallets WHERE user_id = $1 FOR UPDATE`, userID).Scan(&currentBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("wallet not found for user: %s", userID)
		}
		return nil, err
	}
	if !currentBalance.Valid || currentBalance.Float64 < absAmount {
		return nil, fmt.Errorf("insufficient balance: have %v, need %v", currentBalance.Float64, absAmount)
	}

	var newBalance float64
	err = tx.QueryRowContext(ctx, `
UPDATE wallets SET balance = balance - $1, updated_at = NOW()
WHERE user_id = $2
RETURNING balance`, absAmount, userID).Scan(&newBalance)
	if err != nil {
		return nil, err
	}

	var entry LedgerEntry
	err = tx.QueryRowContext(ctx, `
INSERT INTO ledger (user_id, type, amount, balance_after, reference_id, description)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, type, amount, balance_after, reference_id, description, created_at`,
		userID, txType, -absAmount, newBalance, referenceID, description,
	).Scan(&entry.ID, &entry.UserID, &entry.Type, &entry.Amount,
		&entry.BalanceAfter, &entry.ReferenceID, &entry.Description, &entry.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &entry, nil
}
