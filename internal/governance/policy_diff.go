package governance

import (
	"encoding/json"
	"sort"
	"strconv"
)

func buildPolicyDiff(current RuntimePolicy, base *RuntimePolicy) []PolicyDiffEntry {
	currentFlat := flattenRuntimePolicy(current)
	baseFlat := map[string]any{}
	if base != nil {
		baseFlat = flattenRuntimePolicy(*base)
	}

	paths := make(map[string]struct{}, len(currentFlat)+len(baseFlat))
	for path := range currentFlat {
		paths[path] = struct{}{}
	}
	for path := range baseFlat {
		paths[path] = struct{}{}
	}

	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	changes := make([]PolicyDiffEntry, 0, len(sortedPaths))
	for _, path := range sortedPaths {
		baseVal, baseOK := baseFlat[path]
		curVal, curOK := currentFlat[path]
		switch {
		case !baseOK && curOK:
			changes = append(changes, PolicyDiffEntry{Path: path, ChangeType: "added", To: curVal})
		case baseOK && !curOK:
			changes = append(changes, PolicyDiffEntry{Path: path, ChangeType: "removed", From: baseVal})
		case baseOK && curOK && !jsonValueEqual(baseVal, curVal):
			changes = append(changes, PolicyDiffEntry{Path: path, ChangeType: "modified", From: baseVal, To: curVal})
		}
	}
	return changes
}

func flattenRuntimePolicy(policy RuntimePolicy) map[string]any {
	result := map[string]any{}
	walkAny("", normalizeValue(policy), result)
	return result
}

func walkAny(path string, value any, out map[string]any) {
	switch v := value.(type) {
	case map[string]any:
		if len(v) == 0 {
			if path != "" {
				out[path] = map[string]any{}
			}
			return
		}
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			next := k
			if path != "" {
				next = path + "." + k
			}
			walkAny(next, normalizeValue(v[k]), out)
		}
	case []any:
		if len(v) == 0 {
			if path != "" {
				out[path] = []any{}
			}
			return
		}
		for idx, item := range v {
			next := path + "[" + strconv.Itoa(idx) + "]"
			walkAny(next, normalizeValue(item), out)
		}
	default:
		out[path] = v
	}
}

func normalizeValue(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return v
	}
	return normalized
}

func jsonValueEqual(a, b any) bool {
	aRaw, errA := json.Marshal(a)
	bRaw, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(aRaw) == string(bRaw)
}
