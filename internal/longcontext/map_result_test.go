package longcontext

import "testing"

func TestValidateMapResultRequiresVerbatimQuote(t *testing.T) {
	chunk := Chunk{StartChar: 0, EndChar: 20, Content: "alpha beta gamma"}
	good := MapResult{Claims: []MapClaim{{Claim: "fact", Quote: "beta", StartChar: 6, EndChar: 10, Confidence: .9}}}
	if err := ValidateMapResult(good, chunk); err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Claims = []MapClaim{{Claim: "fact", Quote: "not source", StartChar: 6, EndChar: 10, Confidence: .9}}
	if err := ValidateMapResult(bad, chunk); err == nil {
		t.Fatal("accepted non-verbatim quote")
	}
}

func TestValidateMapResultRejectsOutOfRangeSpan(t *testing.T) {
	chunk := Chunk{StartChar: 10, EndChar: 20, Content: "alpha beta gamma"}
	result := MapResult{Claims: []MapClaim{{Claim: "fact", Quote: "delta", StartChar: 0, EndChar: 4, Confidence: .9}}}
	if err := ValidateMapResult(result, chunk); err == nil {
		t.Fatal("accepted out-of-range span")
	}
}

func TestValidateMapResultRecoversSpanFromQuote(t *testing.T) {
	// Quote exists in the chunk but the model supplied an out-of-range span.
	// The validator should recover the absolute span from the quote position.
	chunk := Chunk{StartChar: 100, EndChar: 116, Content: "alpha beta gamma"}
	result := MapResult{Claims: []MapClaim{{Claim: "fact", Quote: "beta", StartChar: 0, EndChar: 4, Confidence: .9}}}
	if err := ValidateMapResult(result, chunk); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
}

func TestValidateMapResultAllowsCaseInsensitiveQuote(t *testing.T) {
	chunk := Chunk{StartChar: 0, EndChar: 20, Content: "Revenue Grew 8 Percent"}
	result := MapResult{Claims: []MapClaim{{Claim: "fact", Quote: "revenue grew 8 percent", StartChar: 0, EndChar: 20, Confidence: .9}}}
	if err := ValidateMapResult(result, chunk); err != nil {
		t.Fatalf("case-insensitive quote rejected: %v", err)
	}
}
