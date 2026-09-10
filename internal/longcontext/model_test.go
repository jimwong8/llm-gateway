package longcontext

import (
	"testing"
)

func TestValidTransitionAllowsLinearPipeline(t *testing.T) {
	cases := [][2]Status{
		{StatusQueued, StatusIngesting},
		{StatusIngesting, StatusMapping},
		{StatusMapping, StatusIndexing},
		{StatusIndexing, StatusRetrieving},
		{StatusRetrieving, StatusSynthesizing},
		{StatusSynthesizing, StatusVerifying},
		{StatusVerifying, StatusSucceeded},
	}
	for _, tc := range cases {
		if !ValidTransition(tc[0], tc[1]) {
			t.Errorf("expected transition %s -> %s to be valid", tc[0], tc[1])
		}
	}
}

func TestTransitionRejectsTerminalAndSkipTransitions(t *testing.T) {
	bad := [][2]Status{
		{StatusSucceeded, StatusQueued},
		{StatusFailed, StatusQueued},
		{StatusQueued, StatusSucceeded},
		{StatusMapping, StatusSucceeded},
	}
	for _, tc := range bad {
		if err := Transition(tc[0], tc[1]); err == nil {
			t.Errorf("expected transition %s -> %s to fail", tc[0], tc[1])
		}
	}
}

func TestValidateCreateRejectsUnsupportedModelAndBounds(t *testing.T) {
	base := CreateTaskInput{Model: "wrong", Mode: "qa", Query: "q", WorkerCount: 1, RetrievalTopK: 1, FinalBudget: 16000}
	if err := ValidateCreate(base, 100, 10); err == nil {
		t.Fatal("expected unsupported model error")
	}
	base.Model = "virtual-long-1m"
	base.WorkerCount = 11
	if err := ValidateCreate(base, 100, 10); err == nil {
		t.Fatal("expected worker bound error")
	}
}
