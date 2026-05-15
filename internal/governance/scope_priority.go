package governance

// ScopePriorityOrder 返回治理配置的固定作用域优先级（高 -> 低）。
// 该顺序用于确定性匹配：emergency override > tenant+agent > tenant > agent > task_type > environment > global。
func ScopePriorityOrder() []string {
	return []string{
		"emergency_override",
		"tenant_agent",
		"tenant",
		"agent",
		"task_type",
		"environment",
		"global",
	}
}
