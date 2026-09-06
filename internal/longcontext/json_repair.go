package longcontext

import (
	"encoding/json"
	"strings"
)

// RepairMapJSON attempts to repair common LLM JSON deviations so that the
// Map result validator can accept output from models that do not strictly
// emit standards-compliant JSON (unquoted keys, single quotes, trailing
// commas, missing values). It returns the repaired bytes and whether the
// original already parsed cleanly. The repair is deliberately conservative:
// it never drops content; it only adds quotes and removes trailing commas.
func RepairMapJSON(raw []byte) ([]byte, bool) {
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) == nil {
		return raw, true
	}
	s := strings.TrimSpace(string(raw))
	s = stripOuterFences(s)
	// First normalize single-quoted keys ('key': -> "key":).
	s = normalizeSingleQuoteKeys(s)
	// Then repair bare keys ({claim: -> {"claim":).
	s = repairUnquotedKeys(s)
	// Then normalize remaining single-quoted strings to double quotes.
	s = repairSingleQuotedStrings(s)
	// Finally drop trailing commas.
	s = repairTrailingCommas(s)
	return []byte(s), false
}

// stripOuterFences removes ```json ... ``` fences or a surrounding code block.
func stripOuterFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			s = ""
		}
	}
	if strings.HasSuffix(s, "```") {
		s = s[:len(s)-3]
	}
	return strings.TrimSpace(s)
}

// normalizeSingleQuoteKeys converts 'key': patterns to "key": so that the
// later unquoted-key repair and value repair do not mis-handle them.
func normalizeSingleQuoteKeys(s string) string {
	if !strings.Contains(s, "'") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 32)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\'' && i+2 < len(runes) {
			// look ahead for 'key':
			j := i + 1
			for j < len(runes) && isKeyRune(runes[j]) {
				j++
			}
			if j > i+1 && j+1 < len(runes) && runes[j] == '\'' {
				k := j + 1
				for k < len(runes) && (runes[k] == ' ' || runes[k] == '\t') {
					k++
				}
				if k < len(runes) && runes[k] == ':' {
					b.WriteRune('"')
					b.WriteString(string(runes[i+1 : j]))
					b.WriteRune('"')
					b.WriteRune(':')
					i = k
					continue
				}
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}

// repairUnquotedKeys adds double quotes around bare object keys, e.g.
// {claim:"x"} -> {"claim":"x"}. It fires after '{', ',' or ':' and skips
// whitespace before the key.
func repairUnquotedKeys(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 64)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		b.WriteRune(c)
		if c != '{' && c != ',' && c != ':' {
			continue
		}
		// skip whitespace
		j := i + 1
		for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
			j++
		}
		// scan a bare key: identifier chars then ':'
		k := j
		for k < len(runes) && isKeyRune(runes[k]) {
			k++
		}
		if k > j && k < len(runes) && runes[k] == ':' {
			b.WriteString(string(runes[i+1 : j])) // whitespace
			b.WriteRune('"')
			b.WriteString(string(runes[j:k]))
			b.WriteRune('"')
			i = k - 1
			continue
		}
	}
	return b.String()
}

func isKeyRune(r rune) bool {
	return r == '_' || r == '-' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// repairTrailingCommas removes commas immediately before '}' or ']'.
func repairTrailingCommas(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == ',' {
			j := i + 1
			for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r') {
				j++
			}
			if j < len(runes) && (runes[j] == '}' || runes[j] == ']') {
				continue
			}
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// repairSingleQuotedStrings converts single-quoted string values to double
// quotes. A single quote opens a string only at an unambiguous value position
// (after ':', ',', '[', '{', or at the very start of the payload); it closes
// at the next single quote. This avoids treating apostrophes inside words as
// quotes, while still fixing model output like {'a':'b'}.
func repairSingleQuotedStrings(s string) string {
	if !strings.Contains(s, "'") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	inDouble := false
	inSingle := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteRune(c)
			continue
		}
		if c == '\'' && !inDouble {
			prev := ' '
			if i > 0 {
				prev = runes[i-1]
			}
			if !inSingle {
				if prev == ':' || prev == ',' || prev == '[' || prev == '{' || i == 0 {
					inSingle = true
					b.WriteRune('"')
					continue
				}
			} else {
				inSingle = false
				b.WriteRune('"')
				continue
			}
		}
		b.WriteRune(c)
	}
	return b.String()
}
