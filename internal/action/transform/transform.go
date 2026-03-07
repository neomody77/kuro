// Package transform implements data transformation actions (jq-like operations).
package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JqAction performs basic jq-like operations on JSON data.
// Supports: dot notation, array indexing, pipe, select, length, keys, values, map, first, last.
type JqAction struct{}

func (a *JqAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	expr, _ := params["expr"].(string)
	if expr == "" {
		return nil, fmt.Errorf("transform.jq: 'expr' parameter is required")
	}

	inputStr, _ := params["input"].(string)

	var data any
	if inputStr != "" {
		if err := json.Unmarshal([]byte(inputStr), &data); err != nil {
			// Treat as raw string if not valid JSON.
			data = inputStr
		}
	} else {
		data = params["data"]
	}

	// Split by pipe and apply each stage.
	stages := splitPipe(expr)
	current := data
	var err error
	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		if stage == "" || stage == "." {
			continue
		}
		current, err = applyStage(current, stage)
		if err != nil {
			return nil, fmt.Errorf("transform.jq: %w", err)
		}
	}

	return map[string]any{
		"output": current,
	}, nil
}

func splitPipe(expr string) []string {
	// Split on | but not inside brackets or quotes.
	var parts []string
	depth := 0
	inQuote := false
	start := 0
	for i, ch := range expr {
		switch {
		case ch == '"' && (i == 0 || expr[i-1] != '\\'):
			inQuote = !inQuote
		case !inQuote && (ch == '[' || ch == '('):
			depth++
		case !inQuote && (ch == ']' || ch == ')'):
			depth--
		case !inQuote && depth == 0 && ch == '|':
			parts = append(parts, expr[start:i])
			start = i + 1
		}
	}
	parts = append(parts, expr[start:])
	return parts
}

func applyStage(data any, stage string) (any, error) {
	stage = strings.TrimSpace(stage)

	switch {
	case stage == "length":
		return getLength(data)

	case stage == "keys":
		return getKeys(data)

	case stage == "values":
		return getValues(data)

	case stage == "first":
		arr, ok := toArray(data)
		if !ok || len(arr) == 0 {
			return nil, nil
		}
		return arr[0], nil

	case stage == "last":
		arr, ok := toArray(data)
		if !ok || len(arr) == 0 {
			return nil, nil
		}
		return arr[len(arr)-1], nil

	case stage == "flatten":
		return flatten(data)

	case stage == "type":
		return jsonType(data), nil

	case strings.HasPrefix(stage, "select(") && strings.HasSuffix(stage, ")"):
		return applySelect(data, stage[7:len(stage)-1])

	case strings.HasPrefix(stage, "map(") && strings.HasSuffix(stage, ")"):
		return applyMap(data, stage[4:len(stage)-1])

	case strings.HasPrefix(stage, "[") && strings.HasSuffix(stage, "]"):
		return applyIndex(data, stage[1:len(stage)-1])

	case strings.HasPrefix(stage, "."):
		return applyDot(data, stage)

	default:
		// Try as a field name.
		return applyDot(data, "."+stage)
	}
}

func applyDot(data any, path string) (any, error) {
	// Remove leading dot.
	path = strings.TrimPrefix(path, ".")
	if path == "" {
		return data, nil
	}

	// Handle array construction: [.[] | ...]
	if strings.HasPrefix(path, "[") {
		return applyIndex(data, path[1:len(path)-1])
	}

	// Split on dots for nested access.
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Check for array index.
		if idx := strings.Index(part, "["); idx >= 0 {
			field := part[:idx]
			idxStr := part[idx+1 : len(part)-1]
			if field != "" {
				m, ok := current.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("cannot access field %q on %T", field, current)
				}
				current = m[field]
			}
			var err error
			current, err = applyIndex(current, idxStr)
			if err != nil {
				return nil, err
			}
			continue
		}

		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot access field %q on %T", part, current)
		}
		current = m[part]
	}
	return current, nil
}

func applyIndex(data any, idxStr string) (any, error) {
	arr, ok := toArray(data)
	if !ok {
		return nil, fmt.Errorf("cannot index %T", data)
	}

	// Handle slice notation: start:end.
	if strings.Contains(idxStr, ":") {
		parts := strings.SplitN(idxStr, ":", 2)
		start := 0
		end := len(arr)
		if parts[0] != "" {
			s, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid slice start: %s", parts[0])
			}
			if s < 0 {
				s = len(arr) + s
			}
			start = s
		}
		if parts[1] != "" {
			e, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid slice end: %s", parts[1])
			}
			if e < 0 {
				e = len(arr) + e
			}
			end = e
		}
		if start < 0 {
			start = 0
		}
		if end > len(arr) {
			end = len(arr)
		}
		if start >= end {
			return []any{}, nil
		}
		return arr[start:end], nil
	}

	idx, err := strconv.Atoi(strings.TrimSpace(idxStr))
	if err != nil {
		return nil, fmt.Errorf("invalid index: %s", idxStr)
	}
	if idx < 0 {
		idx = len(arr) + idx
	}
	if idx < 0 || idx >= len(arr) {
		return nil, nil
	}
	return arr[idx], nil
}

func applySelect(data any, condition string) (any, error) {
	arr, ok := toArray(data)
	if !ok {
		// Apply to single item.
		if matchesCondition(data, condition) {
			return data, nil
		}
		return nil, nil
	}

	var result []any
	for _, item := range arr {
		if matchesCondition(item, condition) {
			result = append(result, item)
		}
	}
	return result, nil
}

func matchesCondition(item any, condition string) bool {
	condition = strings.TrimSpace(condition)

	// Handle .field == "value"
	if parts := strings.SplitN(condition, "==", 2); len(parts) == 2 {
		field := strings.TrimSpace(parts[0])
		expected := strings.TrimSpace(parts[1])
		expected = strings.Trim(expected, `"'`)

		val, err := applyDot(item, field)
		if err != nil {
			return false
		}
		return fmt.Sprintf("%v", val) == expected
	}

	// Handle .field != "value"
	if parts := strings.SplitN(condition, "!=", 2); len(parts) == 2 {
		field := strings.TrimSpace(parts[0])
		expected := strings.TrimSpace(parts[1])
		expected = strings.Trim(expected, `"'`)

		val, err := applyDot(item, field)
		if err != nil {
			return false
		}
		return fmt.Sprintf("%v", val) != expected
	}

	// Handle .field | contains("value")
	if strings.Contains(condition, "| contains(") {
		parts := strings.SplitN(condition, "|", 2)
		field := strings.TrimSpace(parts[0])
		arg := extractFuncArg(strings.TrimSpace(parts[1]), "contains")

		val, err := applyDot(item, field)
		if err != nil {
			return false
		}
		return strings.Contains(fmt.Sprintf("%v", val), arg)
	}

	// Handle .field > N, .field < N
	for _, op := range []string{">=", "<=", ">", "<"} {
		if parts := strings.SplitN(condition, op, 2); len(parts) == 2 {
			field := strings.TrimSpace(parts[0])
			numStr := strings.TrimSpace(parts[1])

			val, err := applyDot(item, field)
			if err != nil {
				return false
			}
			valF := toFloat(val)
			numF, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return false
			}
			switch op {
			case ">":
				return valF > numF
			case "<":
				return valF < numF
			case ">=":
				return valF >= numF
			case "<=":
				return valF <= numF
			}
		}
	}

	return false
}

func applyMap(data any, expr string) (any, error) {
	arr, ok := toArray(data)
	if !ok {
		return nil, fmt.Errorf("map requires an array")
	}

	var result []any
	for _, item := range arr {
		val, err := applyStage(item, expr)
		if err != nil {
			continue
		}
		result = append(result, val)
	}
	return result, nil
}

func getLength(data any) (any, error) {
	switch v := data.(type) {
	case []any:
		return len(v), nil
	case map[string]any:
		return len(v), nil
	case string:
		return len(v), nil
	default:
		return 0, nil
	}
}

func getKeys(data any) (any, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("keys requires an object")
	}
	keys := make([]any, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys, nil
}

func getValues(data any) (any, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("values requires an object")
	}
	vals := make([]any, 0, len(m))
	for _, v := range m {
		vals = append(vals, v)
	}
	return vals, nil
}

func flatten(data any) (any, error) {
	arr, ok := toArray(data)
	if !ok {
		return data, nil
	}
	var result []any
	for _, item := range arr {
		if sub, ok := toArray(item); ok {
			result = append(result, sub...)
		} else {
			result = append(result, item)
		}
	}
	return result, nil
}

func jsonType(data any) string {
	switch data.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64, int:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func toArray(data any) ([]any, bool) {
	arr, ok := data.([]any)
	return arr, ok
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	default:
		return 0
	}
}

func extractFuncArg(call, funcName string) string {
	s := strings.TrimPrefix(call, funcName+"(")
	s = strings.TrimSuffix(s, ")")
	s = strings.Trim(s, `"'`)
	return s
}
