package billing

import (
	"fmt"
	"math"
	"sync"
)

type providerDefaultPrice struct {
	InputPricePer1K  float64
	OutputPricePer1K float64
}

type Pricer struct {
	mu              sync.RWMutex
	providerDefaults map[string]providerDefaultPrice
	modelOverrides   map[string]map[string]providerDefaultPrice
}

func NewPricer() *Pricer {
	return &Pricer{
		providerDefaults: make(map[string]providerDefaultPrice),
		modelOverrides:   make(map[string]map[string]providerDefaultPrice),
	}
}

func (p *Pricer) SetDefaultProviderPrice(provider string, inputPricePer1K, outputPricePer1K float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.providerDefaults[provider] = providerDefaultPrice{
		InputPricePer1K:  inputPricePer1K,
		OutputPricePer1K: outputPricePer1K,
	}
}

func (p *Pricer) SetModelPrice(provider, model string, inputPricePer1K, outputPricePer1K float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.modelOverrides[provider] == nil {
		p.modelOverrides[provider] = make(map[string]providerDefaultPrice)
	}
	p.modelOverrides[provider][model] = providerDefaultPrice{
		InputPricePer1K:  inputPricePer1K,
		OutputPricePer1K: outputPricePer1K,
	}
}

func (p *Pricer) GetPrice(provider, model string) (inputPricePer1K, outputPricePer1K float64, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if overrides, ok := p.modelOverrides[provider]; ok {
		if price, ok := overrides[model]; ok {
			return price.InputPricePer1K, price.OutputPricePer1K, nil
		}
	}

	def, ok := p.providerDefaults[provider]
	if !ok {
		return 0, 0, fmt.Errorf("no pricing for provider: %s", provider)
	}
	return def.InputPricePer1K, def.OutputPricePer1K, nil
}

func (p *Pricer) AllPricing() []PricingRow {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var rows []PricingRow
	for prov, def := range p.providerDefaults {
		rows = append(rows, PricingRow{
			Provider:      prov,
			InputPrice1K:  def.InputPricePer1K,
			OutputPrice1K: def.OutputPricePer1K,
			IsDefault:     true,
		})
	}
	for prov, models := range p.modelOverrides {
		for model, price := range models {
			rows = append(rows, PricingRow{
				Provider:      prov,
				Model:         model,
				InputPrice1K:  price.InputPricePer1K,
				OutputPrice1K: price.OutputPricePer1K,
				IsDefault:     false,
			})
		}
	}
	return rows
}

func (p *Pricer) CalculateCost(provider, model string, promptTokens, completionTokens int) float64 {
	inPrice, outPrice, err := p.GetPrice(provider, model)
	if err != nil {
		return 0
	}
	promptCost := (float64(promptTokens) / 1000.0) * inPrice
	completionCost := (float64(completionTokens) / 1000.0) * outPrice
	return math.Round((promptCost+completionCost)*1000000) / 1000000
}
