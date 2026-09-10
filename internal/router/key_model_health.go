package router

import (
	"container/ring"
	"sync"
)

// KeyModelHealth tracks per-(key, model) health for scheduling decisions.
type KeyModelHealth struct {
	mu       sync.Mutex
	requests map[string]int      // "key:model" → count
	success  map[string]int
	ttfts    *ring.Ring          // last 20 TTFT values
}

func NewKeyModelHealth() *KeyModelHealth {
	return &KeyModelHealth{
		requests: make(map[string]int),
		success:  make(map[string]int),
		ttfts:    ring.New(20),
	}
}

func (k *KeyModelHealth) Score(keyID, model string) float64 {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	kmKey := keyID + ":" + model
	reqCount := k.requests[kmKey]
	if reqCount == 0 {
		return 0.5 // no data → neutral
	}
	
	successRate := float64(k.success[kmKey]) / float64(reqCount)
	
	// Calculate median TTFT from ring buffer
	var ttftList []float64
	k.ttfts.Do(func(v interface{}) {
		if v != nil {
			ttftList = append(ttftList, v.(float64))
		}
	})
	
	var latencyPenalty float64
	if len(ttftList) > 0 {
		// Simple median
		sorted := make([]float64, len(ttftList))
		copy(sorted, ttftList)
		for i := 0; i < len(sorted); i++ {
			for j := i + 1; j < len(sorted); j++ {
				if sorted[j] < sorted[i] {
					sorted[i], sorted[j] = sorted[j], sorted[i]
				}
			}
		}
		median := sorted[len(sorted)/2]
		latencyPenalty = float64(median) / 30.0
		if latencyPenalty > 1.0 {
			latencyPenalty = 1.0
		}
	}
	
	return 0.6*successRate + 0.4*(1.0-latencyPenalty)
}

func (k *KeyModelHealth) RecordSuccess(keyID, model string, ttftMs float64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	kmKey := keyID + ":" + model
	k.requests[kmKey]++
	k.success[kmKey]++
	k.ttfts.Value = ttftMs
	k.ttfts = k.ttfts.Next()
}

func (k *KeyModelHealth) RecordFailure(keyID, model string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	
	kmKey := keyID + ":" + model
	k.requests[kmKey]++
}
