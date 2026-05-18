package billing

import (
	"testing"
)

func TestPricingModelOverrideBeatsDefault(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.015, 0.06)

	inPrice, outPrice, err := p.GetPrice("openai", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inPrice != 0.015 {
		t.Fatalf("expected input price 0.015, got %f", inPrice)
	}
	if outPrice != 0.06 {
		t.Fatalf("expected output price 0.06, got %f", outPrice)
	}
}

func TestPricingDefaultWhenNoModelOverride(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)

	inPrice, outPrice, err := p.GetPrice("openai", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inPrice != 0.01 {
		t.Fatalf("expected default input price 0.01, got %f", inPrice)
	}
	if outPrice != 0.03 {
		t.Fatalf("expected default output price 0.03, got %f", outPrice)
	}
}

func TestPricingUnknownProviderError(t *testing.T) {
	p := NewPricer()
	_, _, err := p.GetPrice("unknown", "gpt-4o-mini")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestPricingModelOverrideAfterDefault(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o", 0.05, 0.15)

	inPrice, outPrice, _ := p.GetPrice("openai", "gpt-4o-mini")
	if inPrice != 0.01 {
		t.Fatalf("expected unoverridden model uses default input 0.01, got %f", inPrice)
	}
	if outPrice != 0.03 {
		t.Fatalf("expected unoverridden model uses default output 0.03, got %f", outPrice)
	}
}

func TestPricingCalculateCost(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetModelPrice("openai", "gpt-4o-mini", 0.015, 0.06)

	cost := p.CalculateCost("openai", "gpt-4o-mini", 100, 50)
	expected := (100.0/1000.0)*0.015 + (50.0/1000.0)*0.06
	if cost != expected {
		t.Fatalf("expected cost %f, got %f", expected, cost)
	}
}

func TestPricingCalculateCostWithDefaults(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)

	cost := p.CalculateCost("openai", "gpt-4o-mini", 100, 50)
	expected := (100.0/1000.0)*0.01 + (50.0/1000.0)*0.03
	if cost != expected {
		t.Fatalf("expected cost %f, got %f", expected, cost)
	}
}

func TestPricingCalculateCostUnknownProvider(t *testing.T) {
	p := NewPricer()
	cost := p.CalculateCost("unknown", "gpt-4o-mini", 100, 50)
	if cost != 0 {
		t.Fatalf("expected 0 cost for unknown provider, got %f", cost)
	}
}

func TestPricingSetDefaultProviderPriceUpdate(t *testing.T) {
	p := NewPricer()
	p.SetDefaultProviderPrice("openai", 0.01, 0.03)
	p.SetDefaultProviderPrice("openai", 0.02, 0.06)

	inPrice, outPrice, _ := p.GetPrice("openai", "gpt-4o-mini")
	if inPrice != 0.02 {
		t.Fatalf("expected updated input price 0.02, got %f", inPrice)
	}
	if outPrice != 0.06 {
		t.Fatalf("expected updated output price 0.06, got %f", outPrice)
	}
}
