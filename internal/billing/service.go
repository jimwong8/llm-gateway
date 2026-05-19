package billing

import (
	"context"
	"fmt"
)

type Service struct {
	store  *Store
	pricer *Pricer
}

func NewService(store *Store, pricer *Pricer) *Service {
	return &Service{store: store, pricer: pricer}
}

func (s *Service) Pricer() *Pricer { return s.pricer }

func (s *Service) EnsureWallet(ctx context.Context, userID string) error {
	return s.store.EnsureWallet(ctx, userID)
}

func (s *Service) GetBalance(ctx context.Context, userID string) (float64, error) {
	return s.store.GetBalance(ctx, userID)
}

func (s *Service) Credit(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	return s.store.Credit(ctx, userID, amount, txType, description, referenceID)
}

func (s *Service) Debit(ctx context.Context, userID string, amount float64, txType, description, referenceID string) (*LedgerEntry, error) {
	return s.store.Debit(ctx, userID, amount, txType, description, referenceID)
}

func (s *Service) PreAuthorize(ctx context.Context, userID, provider, model string, estimatedPromptTokens, estimatedCompletionTokens int) (float64, string, error) {
	estCost := s.pricer.CalculateCost(provider, model, estimatedPromptTokens, estimatedCompletionTokens)
	if estCost == 0 {
		return 0, "", nil
	}

	bal, err := s.store.GetBalance(ctx, userID)
	if err != nil {
		return 0, "", err
	}
	if bal < estCost {
		return 0, "", fmt.Errorf("insufficient balance: have %v, need at least %v", bal, estCost)
	}

	return estCost, "", nil
}

func (s *Service) Settle(ctx context.Context, userID, referenceID, provider, model string, actualPromptTokens, actualCompletionTokens int) (*LedgerEntry, error) {
	cost := s.pricer.CalculateCost(provider, model, actualPromptTokens, actualCompletionTokens)
	if cost == 0 {
		return nil, nil
	}
	if referenceID == "" {
		referenceID = fmt.Sprintf("settle-%s-%s-%d", userID, provider, actualPromptTokens)
	}
	return s.store.Debit(ctx, userID, cost, "api_call", fmt.Sprintf("%s/%s: %dp+%dc", provider, model, actualPromptTokens, actualCompletionTokens), referenceID)
}

func (s *Service) ListLedger(ctx context.Context, filter LedgerFilter) ([]LedgerEntry, error) {
	return s.store.ListLedger(ctx, filter)
}

func (s *Service) AllPricing(ctx context.Context) ([]PricingRow, error) {
	return s.store.AllPricing(ctx)
}

func (s *Service) UpsertPricing(ctx context.Context, provider, model string, inputPrice1K, outputPrice1K float64, isDefault bool) error {
	if isDefault {
		s.pricer.SetDefaultProviderPrice(provider, inputPrice1K, outputPrice1K)
	} else {
		s.pricer.SetModelPrice(provider, model, inputPrice1K, outputPrice1K)
	}
	return s.store.UpsertPricing(ctx, provider, model, inputPrice1K, outputPrice1K, isDefault)
}

func (s *Service) LoadPricingFromDB(ctx context.Context) error {
	rows, err := s.store.ListPricing(ctx)
	if err != nil {
		return err
	}
	for _, p := range rows {
		if p.IsDefault {
			s.pricer.SetDefaultProviderPrice(p.Provider, p.InputPrice1K, p.OutputPrice1K)
		} else {
			s.pricer.SetModelPrice(p.Provider, p.Model, p.InputPrice1K, p.OutputPrice1K)
		}
	}
	return nil
}
