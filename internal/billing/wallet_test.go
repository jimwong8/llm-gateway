package billing

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type memWalletStore struct {
	mu       sync.Mutex
	wallets  map[string]float64
	ledger   []LedgerEntry
	nextID   int64
	refs     map[string]bool
}

func newMemWalletStore() *memWalletStore {
	return &memWalletStore{
		wallets: make(map[string]float64),
		ledger:  make([]LedgerEntry, 0),
		nextID:  1,
		refs:    make(map[string]bool),
	}
}

func (m *memWalletStore) EnsureWallet(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.wallets[userID]; !ok {
		m.wallets[userID] = 0
	}
	return nil
}

func (m *memWalletStore) GetBalance(_ context.Context, userID string) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	bal, ok := m.wallets[userID]
	if !ok {
		return 0, nil
	}
	return bal, nil
}

func (m *memWalletStore) Credit(_ context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if referenceID != "" {
		if m.refs[referenceID] {
			for i := len(m.ledger) - 1; i >= 0; i-- {
				if m.ledger[i].ReferenceID == referenceID {
					return &m.ledger[i], nil
				}
			}
		}
	}

	if _, ok := m.wallets[userID]; !ok {
		m.wallets[userID] = 0
	}
	m.wallets[userID] += amount
	newBal := m.wallets[userID]

	id := m.nextID
	m.nextID++
	entry := LedgerEntry{
		ID:           id,
		UserID:       userID,
		Type:         txType,
		Amount:       amount,
		BalanceAfter: newBal,
		ReferenceID:  referenceID,
		Description:  description,
		CreatedAt:    time.Now(),
	}
	m.ledger = append(m.ledger, entry)
	if referenceID != "" {
		m.refs[referenceID] = true
	}
	return &entry, nil
}

func (m *memWalletStore) Debit(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if referenceID != "" {
		if m.refs[referenceID] {
			for i := len(m.ledger) - 1; i >= 0; i-- {
				if m.ledger[i].ReferenceID == referenceID {
					return &m.ledger[i], nil
				}
			}
		}
	}

	bal, ok := m.wallets[userID]
	if !ok {
		bal = 0
	}
	if bal < amount {
		return nil, fmt.Errorf("insufficient balance: have %v, need %v", bal, amount)
	}
	m.wallets[userID] = bal - amount
	newBal := m.wallets[userID]

	id := m.nextID
	m.nextID++
	entry := LedgerEntry{
		ID:           id,
		UserID:       userID,
		Type:         txType,
		Amount:       -amount,
		BalanceAfter: newBal,
		ReferenceID:  referenceID,
		Description:  description,
		CreatedAt:    time.Now(),
	}
	m.ledger = append(m.ledger, entry)
	if referenceID != "" {
		m.refs[referenceID] = true
	}
	return &entry, nil
}

func TestWalletCreditIncreasesBalance(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	err := m.EnsureWallet(ctx, "user-1")
	if err != nil {
		t.Fatalf("EnsureWallet: %v", err)
	}

	entry, err := m.Credit(ctx, "user-1", 100.0, "manual_topup", "test充值", "ref-1")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if entry.Amount != 100.0 {
		t.Fatalf("expected amount 100.0, got %f", entry.Amount)
	}
	if entry.BalanceAfter != 100.0 {
		t.Fatalf("expected balance_after 100.0, got %f", entry.BalanceAfter)
	}

	bal, err := m.GetBalance(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal != 100.0 {
		t.Fatalf("expected balance 100.0, got %f", bal)
	}
}

func TestWalletDebitDecreasesBalance(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	err := m.EnsureWallet(ctx, "user-2")
	if err != nil {
		t.Fatalf("EnsureWallet: %v", err)
	}

	_, err = m.Credit(ctx, "user-2", 50.0, "manual_topup", "充值", "ref-credit-1")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}

	entry, err := m.Debit(ctx, "user-2", 10.0, "api_call", "对话扣费", "ref-debit-1")
	if err != nil {
		t.Fatalf("Debit: %v", err)
	}
	if entry.Amount != -10.0 {
		t.Fatalf("expected amount -10.0, got %f", entry.Amount)
	}
	if entry.BalanceAfter != 40.0 {
		t.Fatalf("expected balance_after 40.0, got %f", entry.BalanceAfter)
	}

	bal, _ := m.GetBalance(ctx, "user-2")
	if bal != 40.0 {
		t.Fatalf("expected balance 40.0, got %f", bal)
	}
}

func TestWalletInsufficientBalanceRejects(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	err := m.EnsureWallet(ctx, "user-3")
	if err != nil {
		t.Fatalf("EnsureWallet: %v", err)
	}

	_, err = m.Credit(ctx, "user-3", 5.0, "manual_topup", "充值", "ref-credit-2")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}

	_, err = m.Debit(ctx, "user-3", 10.0, "api_call", "扣费", "ref-debit-2")
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
}

func TestWalletIdempotencyKeyPreventsDoubleCharge(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	err := m.EnsureWallet(ctx, "user-4")
	if err != nil {
		t.Fatalf("EnsureWallet: %v", err)
	}

	_, err = m.Credit(ctx, "user-4", 100.0, "manual_topup", "充值", "ref-credit-3")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}

	entry1, err := m.Debit(ctx, "user-4", 10.0, "api_call", "扣费1", "ref-unique-1")
	if err != nil {
		t.Fatalf("first Debit: %v", err)
	}

	entry2, err := m.Debit(ctx, "user-4", 10.0, "api_call", "扣费2", "ref-unique-1")
	if err != nil {
		t.Fatalf("second Debit (idempotent): %v", err)
	}

	if entry2.ID != entry1.ID {
		t.Fatalf("expected idempotent result (same ID), got entry1.ID=%d entry2.ID=%d", entry1.ID, entry2.ID)
	}
	if entry2.BalanceAfter != entry1.BalanceAfter {
		t.Fatalf("expected same balance_after, got %f vs %f", entry2.BalanceAfter, entry1.BalanceAfter)
	}
}

func TestWalletGetBalanceZeroForNewUser(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	bal, err := m.GetBalance(ctx, "nonexistent-user")
	if err != nil {
		t.Fatalf("GetBalance for nonexistent: %v", err)
	}
	if bal != 0 {
		t.Fatalf("expected 0 for nonexistent user, got %f", bal)
	}
}

func TestWalletCreditUpdatesBalanceAfter(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	err := m.EnsureWallet(ctx, "user-5")
	if err != nil {
		t.Fatalf("EnsureWallet: %v", err)
	}

	_, err = m.Credit(ctx, "user-5", 30.0, "manual_topup", "充值", "ref-credit-4")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}
	_, err = m.Credit(ctx, "user-5", 20.0, "manual_topup", "充值2", "ref-credit-5")
	if err != nil {
		t.Fatalf("Credit2: %v", err)
	}

	bal, _ := m.GetBalance(ctx, "user-5")
	if bal != 50.0 {
		t.Fatalf("expected balance 50.0, got %f", bal)
	}
}

func TestWalletDebitInsufficientOnExistingWallet(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "user-6")
	_, _ = m.Credit(ctx, "user-6", 100, "manual_topup", "", "c6")
	_, err := m.Debit(ctx, "user-6", 200, "api_call", "", "d6")
	if err == nil {
		t.Fatal("expected insufficient balance error, got nil")
	}
}

func _storesCompileCheck() {
	s := &Store{}
	ctx := context.Background()
	s.EnsureWallet(ctx, "")
	s.GetBalance(ctx, "")
	s.Credit(ctx, "", 0, "", "", "")
	s.Debit(ctx, "", 0, "", "", "")
	_ = fmt.Sprintf("compile check")
}
