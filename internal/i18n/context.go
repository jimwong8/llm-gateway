package i18n

import (
	"context"
	"net/http"
)

type langKey struct{}

func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langKey{}, lang)
}

func LangFromContext(ctx context.Context) string {
	if lang, ok := ctx.Value(langKey{}).(string); ok {
		return lang
	}
	return "zh-CN"
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := AcceptLanguage(r.Header.Get("Accept-Language"))
		ctx := WithLang(r.Context(), lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

