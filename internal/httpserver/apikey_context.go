package httpserver

import "context"

type apiKeyContextKey struct{}

func withAPIKeyID(ctx context.Context, keyID int64) context.Context {
	return context.WithValue(ctx, apiKeyContextKey{}, keyID)
}

func getAPIKeyID(ctx context.Context) int64 {
	id, _ := ctx.Value(apiKeyContextKey{}).(int64)
	return id
}
