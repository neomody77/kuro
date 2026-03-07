package pipeline

import (
	"fmt"
	"regexp"
	"strings"
)

var exprPattern = regexp.MustCompile(`\{\{\s*\$json\.(\w+(?:\.\w+)*)\s*\}\}`)

// ResolveExpr resolves n8n expression strings like "={{ $json.to }}" against input data.
// Non-expression strings (not starting with "=") are returned as-is.
func ResolveExpr(expr string, item NodeItem) string {
	if !strings.HasPrefix(expr, "=") {
		return expr
	}
	tmpl := expr[1:] // strip "="
	return exprPattern.ReplaceAllStringFunc(tmpl, func(match string) string {
		sub := exprPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		return resolveField(item, sub[1])
	})
}

// resolveField resolves a dot-separated field path like "to" or "options.key" from a NodeItem.
func resolveField(item NodeItem, path string) string {
	parts := strings.Split(path, ".")
	var current any = map[string]any(item)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	return fmt.Sprintf("%v", current)
}

// ResolveNodeParams resolves all n8n expressions in a node's parameters against input data.
func ResolveNodeParams(params map[string]any, item NodeItem) map[string]any {
	resolved := make(map[string]any, len(params))
	for k, v := range params {
		resolved[k] = resolveValue(v, item)
	}
	return resolved
}

func resolveValue(v any, item NodeItem) any {
	switch val := v.(type) {
	case string:
		return ResolveExpr(val, item)
	case map[string]any:
		return ResolveNodeParams(val, item)
	case []any:
		result := make([]any, len(val))
		for i, elem := range val {
			result[i] = resolveValue(elem, item)
		}
		return result
	default:
		return v
	}
}
