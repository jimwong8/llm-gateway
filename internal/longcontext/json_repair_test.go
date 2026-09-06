package longcontext

import (
	"encoding/json"
	"testing"
)

func TestRepairMapJSONAcceptsValidJSON(t *testing.T) {
	raw := []byte(`{"claims":[{"claim":"a","quote":"b","start_char":0,"end_char":1,"confidence":0.9}],"chunk_summary":"s"}`)
	out, clean := RepairMapJSON(raw)
	if !clean {
		t.Fatal("valid JSON flagged as dirty")
	}
	if string(out) != string(raw) {
		t.Fatalf("valid JSON modified: %s", out)
	}
}

func TestRepairMapJSONUnquotedKeys(t *testing.T) {
	raw := []byte(`{claim:"a", quote:"b", start_char:0, end_char:1, confidence:0.9}`)
	out, clean := RepairMapJSON(raw)
	if clean {
		t.Fatal("unquoted keys accepted as clean")
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("repaired JSON still invalid: %v; out=%s", err, out)
	}
	if m["claim"] != "a" {
		t.Fatalf("claim not preserved: %v", m)
	}
}

func TestRepairMapJSONTrailingComma(t *testing.T) {
	raw := []byte(`{"claims":[{"claim":"a","quote":"b",}], "chunk_summary":"s",}`)
	out, clean := RepairMapJSON(raw)
	if clean {
		t.Fatal("trailing comma accepted as clean")
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("repaired JSON invalid: %v; out=%s", err, out)
	}
}

func TestRepairMapJSONFences(t *testing.T) {
	raw := []byte("```json\n{\"claims\":[],\"chunk_summary\":\"s\"}\n```")
	out, clean := RepairMapJSON(raw)
	if clean {
		t.Fatal("fenced JSON accepted as clean")
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("repaired fenced JSON invalid: %v; out=%s", err, out)
	}
}

func TestRepairMapJSONSingleQuotes(t *testing.T) {
	raw := []byte(`{'claims':[{'claim':'a','quote':'b','start_char':0,'end_char':1,'confidence':0.9}],'chunk_summary':'s'}`)
	out, clean := RepairMapJSON(raw)
	if clean {
		t.Fatal("single-quoted JSON accepted as clean")
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("repaired single-quote JSON invalid: %v; out=%s", err, out)
	}
}
