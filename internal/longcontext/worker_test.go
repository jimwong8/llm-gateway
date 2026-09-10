package longcontext

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeMapProvider struct {
	response  string
	gotModel  string
	gotPrompt string
}

func (p *fakeMapProvider) ChatCompletion(_ context.Context, model, prompt string, maxTokens int) (string, error) {
	p.gotModel = model
	p.gotPrompt = prompt
	return p.response, nil
}

type fakeMapStore struct {
	saved  bool
	result MapResult
}

func (s *fakeMapStore) SaveMapResult(_ context.Context, chunkID, taskID, model, provider string, result MapResult) error {
	s.saved = true
	s.result = result
	return nil
}

func TestMapWorkerParsesAndPersistsVerifiedResult(t *testing.T) {
	provider := &fakeMapProvider{response: `{"claims":[{"claim":"fact","quote":"beta","start_char":6,"end_char":10,"confidence":0.9}],"chunk_summary":"alpha beta"}`}
	store := &fakeMapStore{}
	w := MapWorker{Provider: provider, Store: store, Model: "test-model", ProviderName: "test-provider"}
	chunk := Chunk{ID: "c1", TaskID: "t1", StartChar: 0, EndChar: 16, Content: "alpha beta gamma"}
	if err := w.Process(context.Background(), chunk); err != nil {
		t.Fatal(err)
	}
	if !store.saved || provider.gotModel != "test-model" || !strings.Contains(provider.gotPrompt, "alpha beta gamma") {
		t.Fatalf("worker did not persist/request correctly")
	}
}
func TestMapWorkerRejectsInvalidJSON(t *testing.T) {
	w := MapWorker{Provider: &fakeMapProvider{response: "not-json"}, Store: &fakeMapStore{}, Model: "m", ProviderName: "p"}
	err := w.Process(context.Background(), Chunk{ID: "c", TaskID: "t", StartChar: 0, EndChar: 4, Content: "text"})
	if err == nil {
		t.Fatal("accepted invalid JSON")
	}
	var syntax *json.SyntaxError
	_ = syntax
}
