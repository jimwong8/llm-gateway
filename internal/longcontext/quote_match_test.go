package longcontext

import "testing"

func TestFindQuoteSpanExactMatch(t *testing.T) {
	chunk := "In fiscal 2025 revenue grew 8 percent."
	start, end, ok := findQuoteSpan(chunk, "revenue grew 8 percent")
	if !ok || start == 0 || end == 0 {
		t.Fatalf("exact match failed: start=%d end=%d ok=%v", start, end, ok)
	}
	if chunk[start:end] != "revenue grew 8 percent" {
		t.Fatalf("span mismatch: got %q", chunk[start:end])
	}
}

func TestFindQuoteSpanCaseInsensitive(t *testing.T) {
	chunk := "In fiscal 2025 Revenue Grew 8 Percent."
	start, end, ok := findQuoteSpan(chunk, "revenue grew 8 percent")
	if !ok {
		t.Fatal("case-insensitive match failed")
	}
	if chunk[start:end] != "Revenue Grew 8 Percent" {
		t.Fatalf("span mismatch: got %q", chunk[start:end])
	}
}

func TestFindQuoteSpanWhitespaceCollapsed(t *testing.T) {
	chunk := "The  company   reported   12.4 billion dollars."
	start, end, ok := findQuoteSpan(chunk, "company reported 12.4 billion")
	if !ok {
		t.Fatal("whitespace-collapsed match failed")
	}
	_ = start
	_ = end
}

func TestFindQuoteSpanNotPresent(t *testing.T) {
	_, _, ok := findQuoteSpan("alpha beta gamma", "delta")
	if ok {
		t.Fatal("should not match absent quote")
	}
}

func TestQuoteMatchesChunkExact(t *testing.T) {
	if !quoteMatchesChunk("hello world", "hello world") {
		t.Fatal("exact match failed")
	}
}

func TestQuoteMatchesChunkFuzzy(t *testing.T) {
	if !quoteMatchesChunk("HELLO WORLD", "hello world") {
		t.Fatal("case-insensitive match failed")
	}
	if !quoteMatchesChunk("hello   world", "hello world") {
		t.Fatal("whitespace match failed")
	}
}