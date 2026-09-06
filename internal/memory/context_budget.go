package memory

import (
	"fmt"
	"sort"
	"strings"
)

// BudgetState is the 4-level context budget state machine.
// Transitions: normal → stretched → critical → stall
// Corresponds to 10.100.1.13 context-stats endpoint (SESSION_MEMORY_RESEARCH.md §6).
type BudgetState string

const (
	BudgetStateNormal    BudgetState = "normal"
	BudgetStateStretched BudgetState = "stretched"
	BudgetStateCritical  BudgetState = "critical"
	BudgetStateStall     BudgetState = "stall"
)

// ContextStats mirrors the 10.100.1.13 /sessions/{id}/context-stats response.
type ContextStats struct {
	ContextWindowTokens  int         `json:"context_window_tokens"`
	ContextUtilization   float64     `json:"context_utilization"`
	BudgetState          BudgetState `json:"budget_state"`
	BudgetRatio          float64     `json:"budget_ratio"`
	SummaryEnabled       bool        `json:"summary_enabled"`
	SummaryAvailable     bool        `json:"summary_available"`
	GraphHits            int         `json:"graph_hits"`
	GraphContextTokens   int         `json:"graph_context_tokens"`
	RetrievedTokens      int         `json:"retrieved_tokens"`
	RecentTokens         int         `json:"recent_tokens"`
	ReplyReserveTokens   int         `json:"reply_reserve_tokens"`
}

// ContextBudget manages token budget allocation with a 4-level state machine.
type ContextBudget struct {
	MaxTokens     int     `json:"max_tokens"`
	ReserveTokens int     `json:"reserve_tokens"`
	MemoryRatio   float64 `json:"memory_ratio"`
	SystemRatio   float64 `json:"system_ratio"`
	HistoryRatio  float64 `json:"history_ratio"`

	// Thresholds for the state machine (0.0–1.0).
	// stretchedThreshold: utilization above this → "stretched"
	// criticalThreshold: utilization above this → "critical"
	// stallThreshold:     utilization above this → "stall" (context must be truncated)
	StretchedThreshold float64 `json:"stretched_threshold"`
	CriticalThreshold  float64 `json:"critical_threshold"`
	StallThreshold     float64 `json:"stall_threshold"`
}

// DefaultContextBudget returns sensible defaults matching 10.100.1.13.
func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxTokens:          8192,
		ReserveTokens:      1024,
		MemoryRatio:        0.2,
		SystemRatio:        0.1,
		HistoryRatio:       0.5,
		StretchedThreshold: 0.60,
		CriticalThreshold:  0.80,
		StallThreshold:     0.95,
	}
}

// BudgetAllocation holds the result of Allocate().
type BudgetAllocation struct {
	SystemTokens    int `json:"system_tokens"`
	MemoryTokens    int `json:"memory_tokens"`
	HistoryTokens   int `json:"history_tokens"`
	ReserveTokens   int `json:"reserve_tokens"`
	AvailableTokens int `json:"available_tokens"`
}

// Allocate distributes available tokens across system / history / memory.
func (cb *ContextBudget) Allocate(historyTokens, systemTokens int) BudgetAllocation {
	available := cb.MaxTokens - cb.ReserveTokens
	if available <= 0 {
		return BudgetAllocation{ReserveTokens: cb.ReserveTokens}
	}

	sys := min(systemTokens, int(float64(available)*cb.SystemRatio))
	remaining := available - sys

	hist := min(historyTokens, int(float64(remaining)*cb.HistoryRatio))
	remaining -= hist

	mem := min(int(float64(remaining)*cb.MemoryRatio), remaining)

	return BudgetAllocation{
		SystemTokens:    sys,
		MemoryTokens:    mem,
		HistoryTokens:   hist,
		ReserveTokens:   cb.ReserveTokens,
		AvailableTokens: available,
	}
}

// ComputeStats evaluates the current session state and returns ContextStats
// with the correct budget_state.  Call this before each LLM call.
//
// Parameters:
//   - recentTokens:    tokens in the recent message window
//   - systemTokens:    tokens in the system prompt
//   - memoryTokens:    tokens consumed by injected memories
//   - graphTokens:     tokens from KG graph context
//   - summaryTokens:   tokens from session summary (0 if none)
func (cb *ContextBudget) ComputeStats(recentTokens, systemTokens, memoryTokens, graphTokens, summaryTokens int) ContextStats {
	used := recentTokens + systemTokens + memoryTokens + graphTokens + summaryTokens
	available := cb.MaxTokens - cb.ReserveTokens
	if available <= 0 {
		available = 1
	}
	utilization := float64(used) / float64(cb.MaxTokens)

	state := cb.stateForUtilization(utilization)

	return ContextStats{
		ContextWindowTokens: cb.MaxTokens,
		ContextUtilization:  round4(utilization),
		BudgetState:         state,
		BudgetRatio:         round4(float64(used) / float64(available)),
		SummaryEnabled:      true,
		SummaryAvailable:    summaryTokens > 0,
		GraphHits:           0, // filled by caller
		GraphContextTokens:  graphTokens,
		RetrievedTokens:     memoryTokens,
		RecentTokens:        recentTokens,
		ReplyReserveTokens:  cb.ReserveTokens,
	}
}

// ShouldTruncate returns true when the budget state is critical or stall,
// signalling that old messages must be summarised or dropped.
func (cb *ContextBudget) ShouldTruncate(stats ContextStats) bool {
	return stats.BudgetState == BudgetStateCritical || stats.BudgetState == BudgetStateStall
}

// ShouldSummarise returns true when the budget state is stretched or worse,
// signalling that session summarisation should be triggered proactively.
func (cb *ContextBudget) ShouldSummarise(stats ContextStats) bool {
	return stats.BudgetState == BudgetStateStretched ||
		stats.BudgetState == BudgetStateCritical ||
		stats.BudgetState == BudgetStateStall
}

func (cb *ContextBudget) stateForUtilization(u float64) BudgetState {
	switch {
	case u >= cb.StallThreshold:
		return BudgetStateStall
	case u >= cb.CriticalThreshold:
		return BudgetStateCritical
	case u >= cb.StretchedThreshold:
		return BudgetStateStretched
	default:
		return BudgetStateNormal
	}
}

// ---- MemorySelector (unchanged logic, now uses ContextBudget) ----

type MemorySelector struct {
	budget ContextBudget
}

func NewMemorySelector(budget ContextBudget) *MemorySelector {
	return &MemorySelector{budget: budget}
}

func (ms *MemorySelector) SelectMemories(memories []HybridSearchResult, budget int) []HybridSearchResult {
	if len(memories) == 0 || budget <= 0 {
		return nil
	}

	sort.Slice(memories, func(i, j int) bool {
		return memories[i].Score > memories[j].Score
	})

	var selected []HybridSearchResult
	remaining := budget
	for _, m := range memories {
		tokens := estimateTokens(m.Content)
		if tokens > remaining {
			continue
		}
		selected = append(selected, m)
		remaining -= tokens
	}

	return selected
}

func (ms *MemorySelector) BuildContext(selected []HybridSearchResult, systemPrompt, userQuery string) string {
	var sb strings.Builder

	if systemPrompt != "" {
		sb.WriteString("System: ")
		sb.WriteString(systemPrompt)
		sb.WriteString("\n\n")
	}

	if len(selected) > 0 {
		sb.WriteString("Relevant memories:\n")
		for i, m := range selected {
			sb.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, m.Content))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("User: ")
	sb.WriteString(userQuery)

	return sb.String()
}

// ---- helpers ----

func estimateTokens(text string) int {
	return len(text) / 4
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
