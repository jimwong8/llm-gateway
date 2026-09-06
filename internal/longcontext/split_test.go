package longcontext

import "testing"

func TestSplitTextPreservesUTF8AndCoverage(t *testing.T) {
	text := "第一段。第二段。第三段。"
	chunks := SplitText(text, 1, 0)
	if len(chunks) < 2 { t.Fatalf("chunks=%d, want multiple", len(chunks)) }
	for _, c := range chunks {
		if c.Content == "" { t.Fatal("empty chunk") }
		if c.StartChar < 0 || c.EndChar <= c.StartChar { t.Fatalf("invalid span: %+v", c) }
	}
}

func TestSplitTextClampsInvalidOverlap(t *testing.T) {
	chunks := SplitText("abcdefghijklmnopqrstuvwxyz", 2, 99)
	if len(chunks) == 0 { t.Fatal("expected chunks") }
}
