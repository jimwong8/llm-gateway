package memory

import (
	"fmt"
	"sort"
	"strings"
)

type ContextBudget struct {
	MaxTokens     int     `json:"max_tokens"`
	ReserveTokens int     `json:"reserve_tokens"`
	MemoryRatio   float64 `json:"memory_ratio"`
	SystemRatio   float64 `json:"system_ratio"`
	HistoryRatio  float64 `json:"history_ratio"`
}

func DefaultContextBudget() ContextBudget {
	return ContextBudget{
		MaxTokens:     8192,
		ReserveTokens: 1024,
		MemoryRatio:   0.2,
		SystemRatio:   0.1,
		HistoryRatio:  0.5,
	}
}

type BudgetAllocation struct {
	SystemTokens  int `json:"system_tokens"`
	MemoryTokens  int `json:"memory_tokens"`
	HistoryTokens int `json:"history_tokens"`
	ReserveTokens int `json:"reserve_tokens"`
	AvailableTokens int `json:"available_tokens"`
}

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

func estimateTokens(text string) int {
	return len(text) / 4
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

