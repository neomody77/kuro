package adk

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/genai"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/neomody77/kuro/internal/skill"
)

// SkillToADKTools converts all skills in the registry to ADK tools.
func SkillToADKTools(registry *skill.Registry) []tool.Tool {
	var tools []tool.Tool
	for _, sk := range registry.List() {
		tools = append(tools, newSkillTool(sk, registry))
	}
	return tools
}

func newSkillTool(sk *skill.Skill, registry *skill.Registry) tool.Tool {
	return &skillToolWrapper{
		skill:    sk,
		registry: registry,
	}
}

// skillToolWrapper wraps a Kuro skill as an ADK tool.
// It implements tool.Tool (public) and the internal FunctionTool + RequestProcessor
// interfaces via structural typing.
type skillToolWrapper struct {
	skill    *skill.Skill
	registry *skill.Registry
}

// --- tool.Tool interface ---

func (t *skillToolWrapper) Name() string        { return t.skill.Name }
func (t *skillToolWrapper) Description() string { return t.skill.Description }
func (t *skillToolWrapper) IsLongRunning() bool { return false }

// --- FunctionTool interface (matched by ADK via type assertion) ---

func (t *skillToolWrapper) Declaration() *genai.FunctionDeclaration {
	decl := &genai.FunctionDeclaration{
		Name:        t.skill.Name,
		Description: t.skill.Description,
	}

	if len(t.skill.Inputs) > 0 {
		props := map[string]*genai.Schema{}
		var required []string
		for _, p := range t.skill.Inputs {
			schemaType := genai.TypeString
			if p.Type != "" {
				schemaType = mapParamType(p.Type)
			}
			props[p.Name] = &genai.Schema{
				Type:        schemaType,
				Description: p.Name,
			}
			if p.Required {
				required = append(required, p.Name)
			}
		}
		decl.Parameters = &genai.Schema{
			Type:       genai.TypeObject,
			Properties: props,
			Required:   required,
		}
	}

	return decl
}

// destructiveSkills lists skill names (or "skill:action" patterns) requiring HITL confirmation.
var destructiveSkills = map[string]bool{
	"shell":             true,
	"credential:delete": true,
	"document:delete":   true,
	"file:write":        true,
	"file:delete":       true,
	"pipeline:delete":   true,
}

func isDestructive(skillName string, params map[string]any) bool {
	if destructiveSkills[skillName] {
		return true
	}
	if action, ok := params["action"].(string); ok {
		return destructiveSkills[skillName+":"+action]
	}
	return false
}

func (t *skillToolWrapper) Run(ctx tool.Context, args any) (map[string]any, error) {
	params, ok := args.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("skill %q: unexpected args type %T", t.skill.Name, args)
	}

	// HITL: check if this is a destructive action
	if isDestructive(t.skill.Name, params) {
		if confirmation := ctx.ToolConfirmation(); confirmation != nil {
			if !confirmation.Confirmed {
				return map[string]any{"status": "rejected", "message": "Action rejected by user"}, nil
			}
			// Confirmed — fall through to execute
		} else {
			// No confirmation yet — request one
			action := t.skill.Name
			if a, ok := params["action"].(string); ok {
				action = t.skill.Name + ":" + a
			}
			hint := fmt.Sprintf("Execute %s?", action)
			if err := ctx.RequestConfirmation(hint, map[string]any{
				"skill":  t.skill.Name,
				"params": params,
			}); err != nil {
				return nil, err
			}
			ctx.Actions().SkipSummarization = true
			return map[string]any{"status": "awaiting_confirmation"}, nil
		}
	}

	result, err := t.registry.Execute(context.Background(), t.skill.Name, params)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}

	// Ensure result is a map
	var out map[string]any
	switch v := result.(type) {
	case map[string]any:
		out = v
	default:
		resultJSON, _ := json.Marshal(v)
		out = map[string]any{"result": json.RawMessage(resultJSON)}
	}

	// Credential isolation: redact secret values in chat context.
	// Pipelines use CredentialResolver directly and are unaffected.
	if t.skill.Name == "credential" {
		if action, _ := params["action"].(string); action == "get" {
			out = redactCredentialData(out)
		}
	}

	return out, nil
}

// --- RequestProcessor interface (matched by ADK via type assertion) ---
// This packs the tool's FunctionDeclaration into the LLM request.

func (t *skillToolWrapper) ProcessRequest(ctx tool.Context, req *model.LLMRequest) error {
	if req.Tools == nil {
		req.Tools = make(map[string]any)
	}
	if _, ok := req.Tools[t.Name()]; ok {
		return fmt.Errorf("duplicate tool: %q", t.Name())
	}
	req.Tools[t.Name()] = t

	if req.Config == nil {
		req.Config = &genai.GenerateContentConfig{}
	}
	decl := t.Declaration()
	if decl == nil {
		return nil
	}

	// Find existing genai.Tool with FunctionDeclarations and append, or create new
	var funcTool *genai.Tool
	for _, gt := range req.Config.Tools {
		if gt != nil && gt.FunctionDeclarations != nil {
			funcTool = gt
			break
		}
	}
	if funcTool == nil {
		req.Config.Tools = append(req.Config.Tools, &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{decl},
		})
	} else {
		funcTool.FunctionDeclarations = append(funcTool.FunctionDeclarations, decl)
	}
	return nil
}

// redactCredentialData replaces secret values in a credential "get" response
// with placeholders, so the LLM never sees actual secrets.
func redactCredentialData(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	if data, ok := out["data"].(map[string]any); ok {
		redacted := make(map[string]any, len(data))
		for k, v := range data {
			s := fmt.Sprintf("%v", v)
			if len(s) <= 4 {
				redacted[k] = "****"
			} else {
				redacted[k] = s[:2] + "****" + s[len(s)-2:]
			}
		}
		out["data"] = redacted
	} else if data, ok := out["data"].(map[string]string); ok {
		redacted := make(map[string]any, len(data))
		for k, v := range data {
			if len(v) <= 4 {
				redacted[k] = "****"
			} else {
				redacted[k] = v[:2] + "****" + v[len(v)-2:]
			}
		}
		out["data"] = redacted
	}
	return out
}

func mapParamType(t string) genai.Type {
	switch t {
	case "number", "float":
		return genai.TypeNumber
	case "integer", "int":
		return genai.TypeInteger
	case "boolean", "bool":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
