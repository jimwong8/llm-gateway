package billing

import "context"

type DayHitRateRow struct {
	Date     string  `json:"date"`
	HitRate  float64 `json:"hit_rate"`
	Requests int64   `json:"requests"`
}

// CacheHitRateByDay: usage_logs 无缓存列, 返回空集(前端显示为0)。安全降级实现。
func (s *Store) CacheHitRateByDay(ctx context.Context, days int) ([]DayHitRateRow, error) {
	return []DayHitRateRow{}, nil
}


