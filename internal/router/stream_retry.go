package router

import (
	"context"
	"log/slog"
)

type StreamChunk struct {
	Data []byte
	Err  error
	Done bool
}

type StreamErrorChecker func(data []byte) error

func CheckFirstStreamChunkForError(ctx context.Context, src <-chan StreamChunk, checker StreamErrorChecker) (<-chan StreamChunk, <-chan struct{}, error) {
	first, ok := <-src
	if !ok {
		out := make(chan StreamChunk)
		close(out)
		done := make(chan struct{})
		close(done)
		return out, done, nil
	}

	if checker != nil {
		if err := checker(first.Data); err != nil {
			// Drain src in background to prevent goroutine leak
			done := make(chan struct{})
			go func() {
				defer close(done)
				defer func() {
					if rec := recover(); rec != nil {
						slog.Error("stream drain panic", "err", rec)
					}
				}()
				for range src {
				}
			}()
			return nil, done, err
		}
	}

	out := make(chan StreamChunk, 1)
	out <- first

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(out)
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("stream relay panic", "err", rec)
			}
		}()
		for {
			select {
			case chunk, ok := <-src:
				if !ok {
					return
				}
				select {
				case out <- chunk:
				case <-ctx.Done():
					// Drain remaining
					for range src {
					}
					return
				}
			case <-ctx.Done():
				// Drain remaining
				for range src {
				}
				return
			}
		}
	}()

	return out, done, nil
}
