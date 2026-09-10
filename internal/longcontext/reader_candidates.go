package longcontext

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// ReaderCandidate is one model@channel pair eligible to act as the final
// reader (synthesizer) for a 1Mvir task.
type ReaderCandidate struct {
	Model   string
	Channel string
}

// ParseReaderCandidates parses "model@channel,model@channel" strings. Lines
// without @ are skipped so a bare model name cannot silently become a
// channel-less candidate.
func ParseReaderCandidates(s string) []ReaderCandidate {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []ReaderCandidate
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		at := strings.LastIndex(part, "@")
		if at <= 0 || at == len(part)-1 {
			continue
		}
		out = append(out, ReaderCandidate{Model: strings.TrimSpace(part[:at]), Channel: strings.TrimSpace(part[at+1:])})
	}
	return out
}

// synthesizeWithCandidates runs the final-reader (grounded synthesis) step.
// When ReaderCandidates is configured it races them in parallel and uses the
// first candidate that returns a non-empty, evidence-citing answer. The
// legacy single readerChannel/readerModel path is preserved when no candidate
// list is set.
func (p *FullTaskProcessor) synthesizeWithCandidates(ctx context.Context, query string, selected []Evidence, chunks []string, readerChannel, readerModel string) (string, error) {
	candidates := p.readerCandidateList(readerChannel, readerModel)
	if len(candidates) == 1 {
		// Fast path: single reader.
		reader := RegistryFinalReader{
			Registry:        p.Registry,
			Model:           candidates[0].Model,
			Channel:         candidates[0].Channel,
			MaxTokens:       4096,
			ReasoningEffort: "none",
		}
		return reader.SynthesizeGroundedAnswer(ctx, query, selected, chunks)
	}

	// Parallel race: launch each candidate with its own context; first valid
	// non-empty result wins; the rest are cancelled.
	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type outcome struct {
		answer string
		err    error
		model  string
	}
	results := make(chan outcome, len(candidates))
	var wg sync.WaitGroup
	for _, cand := range candidates {
		wg.Add(1)
		go func(c ReaderCandidate) {
			defer wg.Done()
			reader := RegistryFinalReader{
				Registry:        p.Registry,
				Model:           c.Model,
				Channel:         c.Channel,
				MaxTokens:       4096,
				ReasoningEffort: "none",
			}
			start := time.Now()
			ans, err := reader.SynthesizeGroundedAnswer(raceCtx, query, selected, chunks)
			select {
			case results <- outcome{answer: ans, err: err, model: c.Model}:
			default:
			}
			if err == nil {
				slog.Info("1Mvir final-reader candidate finished", "model", c.Model, "channel", c.Channel, "latency_s", time.Since(start).Seconds(), "answer_chars", len([]rune(ans)))
			} else {
				slog.Warn("1Mvir final-reader candidate failed", "model", c.Model, "channel", c.Channel, "err", err)
			}
		}(cand)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var firstErr error
	deadline := time.After(5 * time.Minute)
	for {
		select {
		case <-deadline:
			cancel()
			if firstErr != nil {
				return "", fmt.Errorf("final reader race timed out; last error: %w", firstErr)
			}
			return "", fmt.Errorf("final reader race timed out")
		case res, ok := <-results:
			if !ok {
				if firstErr != nil {
					return "", firstErr
				}
				return "", fmt.Errorf("all final-reader candidates failed")
			}
			if res.err == nil && strings.TrimSpace(res.answer) != "" {
				cancel() // stop the other goroutines
				return res.answer, nil
			}
			if res.err != nil {
				firstErr = res.err
			}
		}
	}
}

// readerCandidateList returns the configured parallel candidate list, or a
// single legacy candidate derived from readerChannel/readerModel when no list
// is configured.
func (p *FullTaskProcessor) readerCandidateList(readerChannel, readerModel string) []ReaderCandidate {
	if len(p.ReaderCandidates) > 0 {
		return p.ReaderCandidates
	}
	return []ReaderCandidate{{Model: readerModel, Channel: readerChannel}}
}
