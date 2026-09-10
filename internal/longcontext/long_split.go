package longcontext

import (
	"strings"
)

// SplitText conservatively splits UTF-8 text by an estimated token budget.
// The first implementation uses a 4-bytes-per-token estimate and never splits
// inside a UTF-8 rune. Overlap is measured in runes for deterministic recovery.
func SplitText(text string, chunkTokens, overlapTokens int) []struct {
	Ordinal, StartChar, EndChar int
	Content                     string
} {
	if text == "" || chunkTokens <= 0 {
		return nil
	}
	runes := []rune(text)
	chunkRunes := chunkTokens * 4
	overlapRunes := overlapTokens * 4
	if chunkRunes < 1 {
		chunkRunes = 1
	}
	if overlapRunes >= chunkRunes {
		overlapRunes = chunkRunes / 4
	}
	step := chunkRunes - overlapRunes
	out := make([]struct {
		Ordinal, StartChar, EndChar int
		Content                     string
	}, 0, (len(runes)+step-1)/step)
	for start, ordinal := 0, 0; start < len(runes); ordinal++ {
		end := start + chunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		content := strings.TrimSpace(string(runes[start:end]))
		if content != "" {
			out = append(out, struct {
				Ordinal, StartChar, EndChar int
				Content                     string
			}{ordinal, start, end, content})
		}
		if end == len(runes) {
			break
		}
		start += step
	}
	return out
}
