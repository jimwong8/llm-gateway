package longcontext

import (
	"strings"
	"unicode"
)

// normalizeForMatch lowercases and collapses all whitespace so that quotes
// that differ only in case or spacing still match their source text.
func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.Join(strings.Fields(s), " ")
}

// findQuoteSpan locates a quote inside the chunk and returns the actual byte
// span. It tries an exact substring match first, then a fuzzy match that is
// case-insensitive and whitespace-insensitive. ok is false when the quote is
// not present even after normalization.
func findQuoteSpan(chunkContent, quote string) (start, end int64, ok bool) {
	if idx := strings.Index(chunkContent, quote); idx >= 0 {
		return int64(idx), int64(idx + len(quote)), true
	}
	normQuote := normalizeForMatch(quote)
	if normQuote == "" {
		return 0, 0, false
	}
	qRunes := []rune(normQuote)
	chunkRunes := []rune(chunkContent)
	for i := 0; i < len(chunkRunes); i++ {
		if isSpaceRune(chunkRunes[i]) {
			continue
		}
		var acc strings.Builder
		lastSpace := false
		for j := i; j < len(chunkRunes); j++ {
			r := chunkRunes[j]
			if isSpaceRune(r) {
				if acc.Len() > 0 && !lastSpace {
					acc.WriteByte(' ')
				}
				lastSpace = true
			} else {
				acc.WriteRune(unicode.ToLower(r))
				lastSpace = false
			}
			if acc.Len() > len(qRunes) {
				break
			}
			if acc.Len() == len(qRunes) && acc.String() == normQuote {
				start = int64(len([]byte(string(chunkRunes[:i]))))
				end = int64(len([]byte(string(chunkRunes[:j+1]))))
				return start, end, true
			}
		}
	}
	return 0, 0, false
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// quoteMatchesChunk reports whether a quote is present in the chunk, allowing
// case and whitespace differences.
func quoteMatchesChunk(chunkContent, quote string) bool {
	if quote == "" {
		return false
	}
	if strings.Contains(chunkContent, quote) {
		return true
	}
	_, _, ok := findQuoteSpan(chunkContent, quote)
	return ok
}
