package main

import (
	"fmt"
	"log"
	"os"

	"llm-gateway/gateway/internal/providers"
	"llm-gateway/gateway/internal/router"
)

func main() {
	r := router.New("default-prov", "default-model")
	
	// Example 1: LoadBalance Policy
	fmt.Println("=== Example 1: LoadBalance Policy ===")
	lbConfig := []byte(`{
		"type": "load_balance",
		"weights": {
			"claude-sonnet": 0.8,
			"gpt-4o-mini": 0.2
		}
	}`)
	lbp, err := router.ParsePolicyConfig(lbConfig)
	if err != nil {
		log.Fatal(err)
	}
	r.SetGlobalPolicy(lbp)

	req1 := providers.ChatCompletionRequest{
		Messages: []providers.ChatMessage{{Content: "Hello!"}},
	}
	
	claudeCount := 0
	gptCount := 0
	for i := 0; i < 1000; i++ {
		d := r.Decide(req1)
		if d.Model == "claude-sonnet" {
			claudeCount++
		} else if d.Model == "gpt-4o-mini" {
			gptCount++
		}
	}
	fmt.Printf("1000 requests using LB Policy (80%% claude, 20%% gpt):\n")
	fmt.Printf("  claude-sonnet matched: %d times\n", claudeCount)
	fmt.Printf("  gpt-4o-mini matched: %d times\n\n", gptCount)

	// Example 2: Fallback Policy
	fmt.Println("=== Example 2: Fallback Policy ===")
	fbConfig := []byte(`{
		"type": "fallback",
		"targets": [
			{
				"type": "direct",
				"model": "fail-code"
			},
			{
				"type": "direct",
				"model": "deepseek-coder"
			}
		]
	}`)
	
	fbp, err := router.ParsePolicyConfig(fbConfig)
	if err != nil {
		log.Fatal(err)
	}
	r.SetGlobalPolicy(fbp)
	
	req2 := providers.ChatCompletionRequest{
		TaskHint: "code",
	}
	// r.Decide actually never executes the fallback if it's not a real API call failure in this test
	// But our FallbackPolicy logic handles failures from children. Since DirectPolicy always succeeds, 
	// the first one will be chosen here.
	d2 := r.Decide(req2)
	fmt.Printf("Fallback decided on: %s (Reason: %s)\n", d2.Model, d2.Reason)
	fmt.Printf("Note: In a real system, the fallback is triggered during the API call, not just routing.\n\n")

	fmt.Println("Done testing routing logic.")
	os.Exit(0)
}
