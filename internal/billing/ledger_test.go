package billing

import (
	"context"
	"testing"
)

func TestLedgerRecordsCreditEntry(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "u1")

	entry, err := m.Credit(ctx, "u1", 50, "manual_topup", "admin充值", "ledger-ref-1")
	if err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if entry.UserID != "u1" {
		t.Fatalf("expected user u1, got %s", entry.UserID)
	}
	if entry.Type != "manual_topup" {
		t.Fatalf("expected type manual_topup, got %s", entry.Type)
	}
	if entry.Amount != 50 {
		t.Fatalf("expected amount 50, got %f", entry.Amount)
	}
	if entry.BalanceAfter != 50 {
		t.Fatalf("expected balance_after 50, got %f", entry.BalanceAfter)
	}
	if entry.Description != "admin充值" {
		t.Fatalf("expected description 'admin充值', got %s", entry.Description)
	}
}

func TestLedgerRecordsDebitEntry(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "u2")
	_, _ = m.Credit(ctx, "u2", 100, "manual_topup", "", "lr2-c")

	entry, err := m.Debit(ctx, "u2", 30, "api_call", "gpt-4o调用", "ledger-ref-2")
	if err != nil {
		t.Fatalf("Debit: %v", err)
	}
	if entry.Amount != -30 {
		t.Fatalf("expected amount -30, got %f", entry.Amount)
	}
	if entry.BalanceAfter != 70 {
		t.Fatalf("expected balance_after 70, got %f", entry.BalanceAfter)
	}
	if entry.Type != "api_call" {
		t.Fatalf("expected type api_call, got %s", entry.Type)
	}
}

func TestLedgerMultipleEntriesPreserveOrder(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "u3")

	e1, _ := m.Credit(ctx, "u3", 100, "manual_topup", "", "lr3-a")
	e2, _ := m.Debit(ctx, "u3", 20, "api_call", "", "lr3-b")
	e3, _ := m.Credit(ctx, "u3", 50, "refund", "", "lr3-c")

	if e1.ID >= e2.ID || e2.ID >= e3.ID {
		t.Fatalf("expected IDs in ascending order: %d < %d < %d", e1.ID, e2.ID, e3.ID)
	}
}

func TestLedgerRefundUsesCredit(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "u4")
	_, _ = m.Credit(ctx, "u4", 100, "manual_topup", "", "lr4-a")
	_, _ = m.Debit(ctx, "u4", 30, "api_call", "", "lr4-b")

	refund, err := m.Credit(ctx, "u4", 30, "refund", "调用退款", "lr4-c")
	if err != nil {
		t.Fatalf("Refund (credit): %v", err)
	}
	if refund.Amount != 30 {
		t.Fatalf("expected amount 30, got %f", refund.Amount)
	}
	if refund.Type != "refund" {
		t.Fatalf("expected type refund, got %s", refund.Type)
	}

	bal, _ := m.GetBalance(ctx, "u4")
	if bal != 100 {
		t.Fatalf("expected final balance 100, got %f", bal)
	}
}

func TestLedgerIdempotentCredit(t *testing.T) {
	m := newMemWalletStore()
	ctx := context.Background()
	_ = m.EnsureWallet(ctx, "u5")

	e1, _ := m.Credit(ctx, "u5", 100, "manual_topup", "", "idem-c")
	e2, _ := m.Credit(ctx, "u5", 100, "manual_topup", "", "idem-c")

	if e1.ID != e2.ID {
		t.Fatalf("expected same entry ID for idempotent credit")
	}
	bal, _ := m.GetBalance(ctx, "u5")
	if bal != 100 {
		t.Fatalf("expected balance 100 (not 200), got %f", bal)
	}
}
