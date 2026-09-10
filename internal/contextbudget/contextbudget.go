package contextbudget

type Message struct {
	Role    string
	Content string
}

type Request struct {
	Messages        []Message
	ToolsJSON       string
	MaxOutputTokens int
}

// Estimate is a conservative provider-independent estimate. Four bytes per
// UTF-8 byte is intentionally not used: ASCII-heavy prompts are closer to
// four bytes/token, while CJK and JSON can be denser. The 4-byte divisor plus
// fixed message overhead is a lower-risk routing estimate for this gateway.
func Estimate(req Request) int {
	total := 0
	for _, msg := range req.Messages {
		total += len([]byte(msg.Content))/4 + 4
	}
	total += len([]byte(req.ToolsJSON)) / 4
	if req.MaxOutputTokens > 0 {
		total += req.MaxOutputTokens
	}
	return total
}

func Fits(required, contextWindow int, safety float64) bool {
	if required < 0 || contextWindow <= 0 || safety <= 0 || safety >= 1 {
		return false
	}
	return float64(required) < float64(contextWindow)*safety
}
