package longcontext

import "fmt"

func Transition(from, to Status) error {
	if !ValidTransition(from, to) {
		return fmt.Errorf("invalid long-context status transition: %s -> %s", from, to)
	}
	return nil
}

func ValidateCreate(in CreateTaskInput, maxInputBytes int, inputBytes int) error {
	if in.Model != "virtual-long-1m" {
		return fmt.Errorf("unsupported model: %s", in.Model)
	}
	if in.Mode != "qa" && in.Mode != "summary" && in.Mode != "analysis" {
		return fmt.Errorf("unsupported mode: %s", in.Mode)
	}
	if in.Query == "" {
		return fmt.Errorf("query is required")
	}
	if inputBytes <= 0 || inputBytes > maxInputBytes {
		return fmt.Errorf("input size out of range")
	}
	if in.WorkerCount < 1 || in.WorkerCount > 10 {
		return fmt.Errorf("worker_count out of range")
	}
	if in.RetrievalTopK < 1 || in.RetrievalTopK > 256 {
		return fmt.Errorf("retrieval_top_k out of range")
	}
	if in.FinalBudget < 16000 || in.FinalBudget > 200000 {
		return fmt.Errorf("final_context_budget out of range")
	}
	return nil
}
