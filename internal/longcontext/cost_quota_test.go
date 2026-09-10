package longcontext

import "testing"

func TestEstimateCost(t *testing.T) {
	if got := estimateCost(1000); got != 0.01 {
		t.Fatalf("estimateCost(1000)=%.4f, want 0.01", got)
	}
}
