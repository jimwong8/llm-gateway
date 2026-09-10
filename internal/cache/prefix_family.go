package cache

import (
	"crypto/sha256"
	"encoding/hex"

	"llm-gateway/gateway/internal/providers"
)

// BuildPrefixFamily: 计算请求前缀指纹(纯函数, 不修改请求)。
// The fingerprint covers model + tenant + all messages except the final user
// turn (the current question), so multi-turn conversations with a stable
// system prompt share a provider prompt cache, while different tenants or
// different system prompts never collide.
func BuildPrefixFamily(req providers.ChatCompletionRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Model))
	h.Write([]byte(req.TenantID))
	msgs := req.Messages
	end := len(msgs)
	if end > 0 && msgs[end-1].Role == "user" {
		end-- // exclude the current user tail; it changes every turn
	}
	for i := 0; i < end; i++ {
		h.Write([]byte(msgs[i].Role))
		h.Write([]byte(msgs[i].Content))
		h.Write([]byte{0}) // role/content boundary
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
