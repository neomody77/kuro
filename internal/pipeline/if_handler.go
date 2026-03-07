package pipeline

import (
	"context"
	"strings"
)

// IfHandler implements the n8n If node — evaluates conditions and routes items
// to output 0 (true) or output 1 (false).
type IfHandler struct{}

func (h *IfHandler) ExecuteNode(_ context.Context, node *Node, input []NodeItem, _ map[string]map[string]string) (*NodeOutput, error) {
	out := &NodeOutput{Items: make([][]NodeItem, 2)} // [0]=true, [1]=false

	condCfg, _ := node.Parameters["conditions"].(map[string]any)
	combinator, _ := condCfg["combinator"].(string)
	if combinator == "" {
		combinator = "and"
	}
	rawConditions, _ := condCfg["conditions"].([]any)
	opts, _ := condCfg["options"].(map[string]any)
	caseSensitive := true
	if cs, ok := opts["caseSensitive"].(bool); ok {
		caseSensitive = cs
	}

	for _, item := range input {
		match := evaluateConditions(rawConditions, combinator, caseSensitive, item)
		if match {
			out.Items[0] = append(out.Items[0], item)
		} else {
			out.Items[1] = append(out.Items[1], item)
		}
	}
	return out, nil
}

func evaluateConditions(rawConditions []any, combinator string, caseSensitive bool, item NodeItem) bool {
	if len(rawConditions) == 0 {
		return true
	}
	for _, rc := range rawConditions {
		cond, ok := rc.(map[string]any)
		if !ok {
			continue
		}
		result := evaluateSingleCondition(cond, caseSensitive, item)
		if combinator == "or" && result {
			return true
		}
		if combinator == "and" && !result {
			return false
		}
	}
	return combinator == "and"
}

func evaluateSingleCondition(cond map[string]any, caseSensitive bool, item NodeItem) bool {
	leftRaw, _ := cond["leftValue"].(string)
	rightRaw, _ := cond["rightValue"].(string)

	left := ResolveExpr(leftRaw, item)
	right := ResolveExpr(rightRaw, item)

	if !caseSensitive {
		left = strings.ToLower(left)
		right = strings.ToLower(right)
	}

	op, _ := cond["operator"].(map[string]any)
	operation, _ := op["operation"].(string)

	switch operation {
	case "contains":
		return strings.Contains(left, right)
	case "notContains":
		return !strings.Contains(left, right)
	case "equals":
		return left == right
	case "notEquals":
		return left != right
	case "startsWith":
		return strings.HasPrefix(left, right)
	case "endsWith":
		return strings.HasSuffix(left, right)
	case "isEmpty":
		return left == ""
	case "isNotEmpty":
		return left != ""
	default:
		return false
	}
}
