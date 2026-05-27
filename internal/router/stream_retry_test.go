package router

import (
	"context"
	"testing"
	"time"
)

func TestCheckFirstStreamChunkForError_ReturnsErrorBeforeExposingChunk(t *testing.T) {
	ctx := context.Background()
	src := make(chan StreamChunk, 1)
	src <- StreamChunk{Data: []byte(`{"error":{"message":"rate limited","type":"rate_limit"}}`), Err: newFakeHTTPError(429, "rate limited")}
	close(src)

	out, drainDone, firstErr := CheckFirstStreamChunkForError(ctx, src, func(data []byte) error {
		// Simulate parsing the error from SSE data
		return newFakeHTTPError(429, "rate limited")
	})

	if firstErr == nil {
		t.Fatal("expected error from first chunk")
	}
	if out != nil {
		t.Fatal("expected nil output channel on error")
	}
	<-drainDone // ensure drain completes
}

func TestCheckFirstStreamChunkForError_ForwardsFirstChunkOnSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	src := make(chan StreamChunk, 2)
	src <- StreamChunk{Data: []byte(`{"choices":[{"delta":{"content":"hello"}}]}`)}
	src <- StreamChunk{Data: []byte(`{"choices":[{"delta":{"content":" world"}}]}`)}
	close(src)

	out, drainDone, firstErr := CheckFirstStreamChunkForError(ctx, src, func(data []byte) error {
		return nil // no error in data
	})

	if firstErr != nil {
		t.Fatalf("unexpected error: %v", firstErr)
	}
	if out == nil {
		t.Fatal("expected non-nil output channel")
	}

	// Verify first chunk is forwarded
	select {
	case chunk := <-out:
		if string(chunk.Data) != `{"choices":[{"delta":{"content":"hello"}}]}` {
			t.Fatalf("unexpected first chunk: %s", chunk.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for first chunk")
	}

	// Drain remaining
	for range out {
	}
	<-drainDone
}

func TestCheckFirstStreamChunkForError_DrainsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	src := make(chan StreamChunk, 1)
	src <- StreamChunk{Data: []byte(`{"choices":[{"delta":{"content":"data"}}]}`)}
	close(src)

	out, drainDone, firstErr := CheckFirstStreamChunkForError(ctx, src, func(data []byte) error {
		return nil
	})

	if firstErr != nil {
		t.Fatalf("unexpected error: %v", firstErr)
	}

	// Cancel context to trigger drain
	cancel()

	// Should complete without hanging
	select {
	case <-drainDone:
		// OK
	case <-time.After(3 * time.Second):
		t.Fatal("drain did not complete after context cancel")
	}
	_ = out
}
