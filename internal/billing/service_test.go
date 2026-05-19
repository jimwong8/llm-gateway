package billing

import (
	"context"
	"testing"
)

func TestServicePreAuthorizePassthroughOnZeroCost(t *testing.T) {
	p := NewPricer()
	svc := &Service{store: nil, pricer: p}

	estCost, _, err := svc.PreAuthorize(context.Background(), "user-1", "unknown", "gpt-4o", 500, 500)
	if err != nil {
		t.Fatalf("expected nil error for unknown provider (zero cost), got: %v", err)
	}
	if estCost != 0 {
		t.Fatalf("expected estCost 0, got %f", estCost)
	}
}

func TestServicePreAuthorizeWithMemStoreLogic(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-1")
	m.Credit(ctx, "user-1", 100, "manual_topup", "", "ref-init")

	estCost := p.CalculateCost("openai", "gpt-4o-mini", 500, 500)
	if estCost <= 0 {
		t.Fatalf("expected positive cost, got %f", estCost)
	}

	bal, err := m.GetBalance(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal < estCost {
		t.Fatalf("insufficient balance for pre-auth: have %v, need %v", bal, estCost)
	}
}

func TestServicePreAuthorizeInsufficientBalance(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.05, 0.15)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-2")
	m.Credit(ctx, "user-2", 0.5, "manual_topup", "", "ref-init")
	m.Credit(ctx, "user-2", 0.0005, "manual_topup", "", "ref-init2")

	estCost := p.CalculateCost("openai", "gpt-4o", 5000, 5000)
	bal, _ := m.GetBalance(ctx, "user-2")
	if bal >= estCost {
		t.Fatalf("test setup error: balance %v should be less than cost %v", bal, estCost)
	}
}

func TestServiceSettleFullFlow(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.0015, 0.006)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-3")
	m.Credit(ctx, "user-3", 100, "manual_topup", "", "ref-init")

	preAuthCost := p.CalculateCost("openai", "gpt-4o-mini", 500, 500)
	bal, _ := m.GetBalance(ctx, "user-3")
	if bal < preAuthCost {
		t.Fatalf("insufficient for pre-auth: have %v, need %v", bal, preAuthCost)
	}

	actualCost := p.CalculateCost("openai", "gpt-4o-mini", 200, 400)
	entry, err := m.Debit(ctx, "user-3", actualCost, "api_call", "openai/gpt-4o-mini: 200p+400c", "req-settle-1")
	if err != nil {
		t.Fatalf("Debit (settle) failed: %v", err)
	}

	finalBal, _ := m.GetBalance(ctx, "user-3")
	expectedBal := 100 - actualCost
	if finalBal != expectedBal {
		t.Fatalf("expected balance %v, got %v", expectedBal, finalBal)
	}
	if entry.Type != "api_call" {
		t.Fatalf("expected tx type api_call, got %s", entry.Type)
	}
	if entry.Amount != -actualCost {
		t.Fatalf("expected amount -%f, got %f", actualCost, entry.Amount)
	}
}

func TestServiceSettleWithIdempotencyKey(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-4")
	m.Credit(ctx, "user-4", 50, "manual_topup", "", "ref-init")

	refID := "idempotent-req-1"
	cost := p.CalculateCost("openai", "gpt-4o-mini", 150, 300)

	entry1, err1 := m.Debit(ctx, "user-4", cost, "api_call", "openai/gpt-4o-mini: 150p+300c", refID)
	if err1 != nil {
		t.Fatalf("first debit: %v", err1)
	}

	entry2, err2 := m.Debit(ctx, "user-4", cost, "api_call", "openai/gpt-4o-mini: 150p+300c", refID)
	if err2 != nil {
		t.Fatalf("second debit (idempotent): %v", err2)
	}

	if entry2.ID != entry1.ID {
		t.Fatalf("expected same entry ID for idempotent settle, got %d vs %d", entry1.ID, entry2.ID)
	}
	if entry2.BalanceAfter != entry1.BalanceAfter {
		t.Fatalf("expected same balance_after for idempotent settle, got %f vs %f", entry2.BalanceAfter, entry1.BalanceAfter)
	}
}

func TestServiceSettleWithZeroCost(t *testing.T) {
	p := NewPricer()

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-5")
	m.Credit(ctx, "user-5", 100, "manual_topup", "", "ref-init")

	cost := p.CalculateCost("unknown", "gpt-4o", 100, 100)
	if cost != 0 {
		t.Fatalf("expected zero cost for unknown provider, got %f", cost)
	}

	balBefore, _ := m.GetBalance(ctx, "user-5")
	if balBefore != 100 {
		t.Fatalf("expected initial balance 100, got %f", balBefore)
	}
}

func TestServiceMultipleSettlesDeductCorrectly(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.0015, 0.006)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-6")
	m.Credit(ctx, "user-6", 100, "manual_topup", "", "ref-init")

	cost1 := p.CalculateCost("openai", "gpt-4o-mini", 200, 400)
	cost2 := p.CalculateCost("openai", "gpt-4o-mini", 100, 200)
	cost3 := p.CalculateCost("openai", "gpt-4o-mini", 300, 600)

	m.Debit(ctx, "user-6", cost1, "api_call", "", "req-multi-1")
	m.Debit(ctx, "user-6", cost2, "api_call", "", "req-multi-2")
	m.Debit(ctx, "user-6", cost3, "api_call", "", "req-multi-3")

	expectedBal := 100 - cost1 - cost2 - cost3
	finalBal, _ := m.GetBalance(ctx, "user-6")
	if finalBal != expectedBal {
		t.Fatalf("expected balance %v, got %v", expectedBal, finalBal)
	}
}

func TestServicePricingConsistency(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.0015, 0.006)

	costPre := p.CalculateCost("openai", "gpt-4o-mini", 200, 400)
	costPost := p.CalculateCost("openai", "gpt-4o-mini", 200, 400)

	if costPre != costPost {
		t.Fatalf("pricing not consistent: %f vs %f", costPre, costPost)
	}
}

func TestServicePreAuthorizeThenSettleFlow(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.0015, 0.006)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-7")
	m.Credit(ctx, "user-7", 100, "manual_topup", "", "ref-init")

	estCost := p.CalculateCost("openai", "gpt-4o-mini", 500, 500)
	bal, _ := m.GetBalance(ctx, "user-7")
	if bal < estCost {
		t.Fatalf("pre-auth would fail: have %v, need %v", bal, estCost)
	}

	actualCost := p.CalculateCost("openai", "gpt-4o-mini", 150, 280)
	entry, err := m.Debit(ctx, "user-7", actualCost, "api_call", "openai/gpt-4o-mini: 150p+280c", "req-full-1")
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}

	finalBal, _ := m.GetBalance(ctx, "user-7")
	if finalBal != 100-actualCost {
		t.Fatalf("expected balance %v, got %v", 100-actualCost, finalBal)
	}

	if entry.ReferenceID != "req-full-1" {
		t.Fatalf("expected reference_id req-full-1, got %s", entry.ReferenceID)
	}
}

func TestServiceSettleCreatesCorrectDescription(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)

	m := newMemWalletStore()
	ctx := context.Background()
	m.EnsureWallet(ctx, "user-8")
	m.Credit(ctx, "user-8", 50, "manual_topup", "", "ref-init")

	cost := p.CalculateCost("openai", "gpt-4o", 100, 200)
	expectedDesc := "openai/gpt-4o: 100p+200c"

	entry, err := m.Debit(ctx, "user-8", cost, "api_call", expectedDesc, "req-desc-1")
	if err != nil {
		t.Fatalf("debit: %v", err)
	}
	if entry.Description != expectedDesc {
		t.Fatalf("expected description %q, got %q", expectedDesc, entry.Description)
	}
}
