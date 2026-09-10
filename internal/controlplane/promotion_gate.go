package controlplane

type GateResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

type ValidationHook interface {
	Name() string
	Validate() error
}

type Failpoint struct {
	Name   string `json:"name"`
	Reason string `json:"reason,omitempty"`
}

func RunValidationHooks(hooks []ValidationHook) []GateResult {
	results := make([]GateResult, 0, len(hooks))
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		err := hook.Validate()
		result := GateResult{Name: hook.Name(), Passed: err == nil}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}
