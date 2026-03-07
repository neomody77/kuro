// Package skill manages the skill registry and skill execution.
package skill

import (
	"context"
	"fmt"
	"sync"

	"github.com/neomody77/kuro/internal/pipeline"
)

// Skill represents a reusable action handler or workflow fragment.
type Skill struct {
	Name        string               `json:"name" yaml:"name"`
	Description string               `json:"description,omitempty" yaml:"description"`
	Inputs      []SkillParam         `json:"inputs,omitempty" yaml:"inputs"`
	Outputs     []SkillParam         `json:"outputs,omitempty" yaml:"outputs"`
	Workflow    *pipeline.Workflow    `json:"workflow,omitempty" yaml:"workflow"`
	Handler     pipeline.ActionHandler `json:"-" yaml:"-"`
}

// SkillParam defines an input or output parameter for a skill.
type SkillParam struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type,omitempty" yaml:"type"`
	Required bool   `json:"required,omitempty" yaml:"required"`
	Default  string `json:"default,omitempty" yaml:"default"`
}

// Registry manages loaded skills and executes them.
type Registry struct {
	mu       sync.RWMutex
	skills   map[string]*Skill
	executor *pipeline.Executor
}

// NewRegistry creates a new skill registry.
func NewRegistry(executor *pipeline.Executor) *Registry {
	return &Registry{
		skills:   make(map[string]*Skill),
		executor: executor,
	}
}

// Register adds a skill to the registry.
func (r *Registry) Register(s *Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[s.Name] = s
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

// List returns all registered skills.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}

// Delete removes a skill from the registry.
func (r *Registry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
}

// Execute runs a skill by name with the given inputs.
// If the skill has a direct Handler, it is called directly.
// Otherwise, the skill's Workflow is executed.
func (r *Registry) Execute(ctx context.Context, skillName string, inputs map[string]any) (any, error) {
	s, ok := r.Get(skillName)
	if !ok {
		return nil, fmt.Errorf("skill: unknown skill %q", skillName)
	}

	// Validate required inputs.
	for _, param := range s.Inputs {
		if param.Required {
			if _, ok := inputs[param.Name]; !ok {
				if param.Default != "" {
					inputs[param.Name] = param.Default
				} else {
					return nil, fmt.Errorf("skill: missing required input %q for skill %q", param.Name, skillName)
				}
			}
		}
	}

	// If the skill has a direct handler, use it.
	if s.Handler != nil {
		return s.Handler.Execute(ctx, inputs, nil)
	}

	// Execute the skill's embedded workflow.
	if s.Workflow == nil {
		return nil, fmt.Errorf("skill: %q has no handler or workflow", skillName)
	}

	if r.executor == nil {
		return nil, fmt.Errorf("skill: no workflow executor configured")
	}

	// Inject inputs into node parameters.
	w := *s.Workflow
	w.Name = "skill:" + skillName
	for i := range w.Nodes {
		for k, v := range w.Nodes[i].Parameters {
			if str, ok := v.(string); ok {
				w.Nodes[i].Parameters[k] = resolveInputs(str, inputs)
			}
		}
	}

	exec, err := r.executor.Execute(ctx, &w)
	if err != nil {
		return nil, fmt.Errorf("skill: execute %q: %w", skillName, err)
	}

	// Collect result from last executed node.
	result := map[string]any{
		"status": string(exec.Status),
	}
	if exec.Data != nil && exec.Data.ResultData.LastNodeExecuted != "" {
		lastNode := exec.Data.ResultData.LastNodeExecuted
		if runs, ok := exec.Data.ResultData.RunData[lastNode]; ok && len(runs) > 0 {
			last := runs[len(runs)-1]
			if last.Data != nil {
				result["output"] = last.Data
			}
		}
	}

	return result, nil
}

// resolveInputs replaces {{ inputs.X }} references in a string with actual input values.
func resolveInputs(tmpl string, inputs map[string]any) string {
	for k, v := range inputs {
		old := fmt.Sprintf("{{ inputs.%s }}", k)
		tmpl = replaceAll(tmpl, old, fmt.Sprintf("%v", v))
		old2 := fmt.Sprintf("{{inputs.%s}}", k)
		tmpl = replaceAll(tmpl, old2, fmt.Sprintf("%v", v))
	}
	return tmpl
}

func replaceAll(s, old, new string) string {
	for {
		i := indexOf(s, old)
		if i < 0 {
			return s
		}
		s = s[:i] + new + s[i+len(old):]
	}
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
