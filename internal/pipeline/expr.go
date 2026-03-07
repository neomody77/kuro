package pipeline

import (
	"fmt"
	"strings"
	"time"
)

// ResolveExpressions replaces {{ ... }} template expressions in a string
// using the provided node results.
func ResolveExpressions(tmpl string, results map[string]*NodeResult) string {
	for {
		start := strings.Index(tmpl, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(tmpl[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		expr := strings.TrimSpace(tmpl[start+2 : end-2])
		value := evaluateExpr(expr, results)
		tmpl = tmpl[:start] + value + tmpl[end:]
	}
	return tmpl
}

// evaluateExpr evaluates a single template expression.
func evaluateExpr(expr string, results map[string]*NodeResult) string {
	// Handle pipe operators: expr | filter
	parts := strings.SplitN(expr, "|", 2)
	base := strings.TrimSpace(parts[0])
	value := resolveRef(base, results)

	if len(parts) == 2 {
		filter := strings.TrimSpace(parts[1])
		value = applyFilter(value, filter)
	}

	return value
}

// resolveRef resolves a dotted reference like "nodes.fetch.output" or "nodes.fetch.messages".
func resolveRef(ref string, results map[string]*NodeResult) string {
	// Handle "now" keyword.
	if ref == "now" {
		return time.Now().Format(time.RFC3339)
	}

	// Handle "nodes.X.output" or "nodes.X.<field>".
	if strings.HasPrefix(ref, "nodes.") {
		parts := strings.SplitN(ref, ".", 3)
		if len(parts) < 3 {
			return ""
		}
		nodeID := parts[1]
		field := parts[2]

		result, ok := results[nodeID]
		if !ok {
			return ""
		}

		switch field {
		case "output":
			return fmt.Sprintf("%v", result.Output)
		case "error":
			return result.Error
		case "status":
			return string(result.Status)
		case "duration":
			return result.Duration.String()
		default:
			// Try to access output as a map.
			if m, ok := result.Output.(map[string]any); ok {
				if v, ok := m[field]; ok {
					return fmt.Sprintf("%v", v)
				}
			}
			return ""
		}
	}

	return ref
}

// applyFilter applies a pipe filter to a value.
func applyFilter(value string, filter string) string {
	filter = strings.TrimSpace(filter)

	switch {
	case strings.HasPrefix(filter, "date("):
		// {{ now | date('YYYY-MM-DD') }}
		format := extractQuotedArg(filter, "date")
		return formatDate(value, format)
	case filter == "length":
		return fmt.Sprintf("%d", len(value))
	case strings.HasPrefix(filter, "contains("):
		arg := extractQuotedArg(filter, "contains")
		if strings.Contains(value, arg) {
			return "true"
		}
		return "false"
	case filter == "upper":
		return strings.ToUpper(value)
	case filter == "lower":
		return strings.ToLower(value)
	case filter == "trim":
		return strings.TrimSpace(value)
	}

	return value
}

// extractQuotedArg extracts the quoted argument from a function call like date('YYYY-MM-DD').
func extractQuotedArg(filter string, funcName string) string {
	// Strip "funcName(" prefix and ")" suffix.
	inner := strings.TrimPrefix(filter, funcName+"(")
	inner = strings.TrimSuffix(inner, ")")
	inner = strings.Trim(inner, "'\"")
	return inner
}

// formatDate converts an RFC3339 timestamp to the given format.
func formatDate(value string, format string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t = time.Now()
	}

	// Convert common format tokens to Go layout.
	r := strings.NewReplacer(
		"YYYY", "2006",
		"YY", "06",
		"MM", "01",
		"DD", "02",
		"HH", "15",
		"mm", "04",
		"ss", "05",
	)
	goFmt := r.Replace(format)
	return t.Format(goFmt)
}

// ResolveParams resolves template expressions in a params map.
func ResolveParams(params map[string]any, results map[string]*NodeResult) map[string]any {
	resolved := make(map[string]any, len(params))
	for k, v := range params {
		if s, ok := v.(string); ok {
			resolved[k] = ResolveExpressions(s, results)
		} else {
			resolved[k] = v
		}
	}
	return resolved
}
