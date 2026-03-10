// Package skill manages the skill registry and skill execution.
package skill

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/neomody77/kuro/internal/pipeline"
)

// Skill represents a reusable action handler or workflow fragment.
// Both built-in and external (YAML-defined) skills use the same struct.
type Skill struct {
	Name        string               `json:"name" yaml:"name"`
	Description string               `json:"description,omitempty" yaml:"description"`
	Inputs      []SkillParam         `json:"inputs,omitempty" yaml:"inputs"`
	Outputs     []SkillParam         `json:"outputs,omitempty" yaml:"outputs"`
	Workflow    *pipeline.Workflow    `json:"workflow,omitempty" yaml:"workflow"`
	Handler     pipeline.ActionHandler `json:"-" yaml:"-"`

	// Command is a shell command for external skill execution (stdin JSON → stdout JSON).
	Command string `json:"command,omitempty" yaml:"command,omitempty"`
	// Endpoint is an HTTP URL for external skill execution (POST JSON → JSON response).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	// On lists event types that trigger this skill asynchronously (e.g. "pipeline.failed").
	On []string `json:"on,omitempty" yaml:"on,omitempty"`
	// Require specifies runtime prerequisites (env vars, binaries, OS).
	Require *SkillRequire `json:"require,omitempty" yaml:"require,omitempty"`
	// Destructive marks skills that need user confirmation before execution.
	Destructive bool `json:"destructive,omitempty" yaml:"destructive,omitempty"`
	// Source indicates where the skill was loaded from: "builtin", "global", or "workspace".
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	// Config declares configuration fields this skill needs (e.g. API keys).
	// Values are stored as credentials keyed by "skill:{name}".
	Config []SkillParam `json:"config,omitempty" yaml:"config,omitempty"`
}

// SkillRequire defines runtime prerequisites for a skill.
type SkillRequire struct {
	Env  []string `json:"env,omitempty" yaml:"env,omitempty"`   // required environment variables
	Bins []string `json:"bins,omitempty" yaml:"bins,omitempty"` // required binaries in PATH
	OS   []string `json:"os,omitempty" yaml:"os,omitempty"`     // allowed OS values (runtime.GOOS)
}

// SkillParam defines an input or output parameter for a skill.
type SkillParam struct {
	Name     string `json:"name" yaml:"name"`
	Type     string `json:"type,omitempty" yaml:"type"`
	Required bool   `json:"required,omitempty" yaml:"required"`
	Default  string `json:"default,omitempty" yaml:"default"`
}

// SkillConfigStore is the interface for reading/writing skill configuration.
// Skill configs are stored as credentials keyed by "skill:{name}".
type SkillConfigStore interface {
	GetConfig(skillName string) (map[string]string, error)
	SaveConfig(skillName string, data map[string]string) error
}

// Registry manages loaded skills and executes them.
type Registry struct {
	mu          sync.RWMutex
	skills      map[string]*Skill
	executor    *pipeline.Executor
	configStore SkillConfigStore
}

// NewRegistry creates a new skill registry.
func NewRegistry(executor *pipeline.Executor) *Registry {
	return &Registry{
		skills:   make(map[string]*Skill),
		executor: executor,
	}
}

// SetConfigStore sets the credential-backed config store for skill configuration.
func (r *Registry) SetConfigStore(store SkillConfigStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configStore = store
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

// ListByEvent returns all skills triggered by the given event type.
func (r *Registry) ListByEvent(eventType string) []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*Skill
	for _, s := range r.skills {
		for _, on := range s.On {
			if on == eventType {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

// ExecuteEvent runs all skills matching the given event type asynchronously.
// Errors are logged, not returned. Used by HookDispatcher.
func (r *Registry) ExecuteEvent(ctx context.Context, eventType string, params map[string]any) {
	skills := r.ListByEvent(eventType)
	for _, sk := range skills {
		go func(s *Skill) {
			_, err := r.Execute(ctx, s.Name, params)
			if err != nil {
				log.Printf("skill: event hook %q (trigger %q) failed: %v", s.Name, eventType, err)
			}
		}(sk)
	}
}

// Execute runs a skill by name with the given inputs.
// If the skill has a direct Handler, it is called directly.
// Otherwise, the skill's Workflow is executed.
func (r *Registry) Execute(ctx context.Context, skillName string, inputs map[string]any) (any, error) {
	s, ok := r.Get(skillName)
	if !ok {
		return nil, fmt.Errorf("skill: unknown skill %q", skillName)
	}

	// Check runtime requirements before execution.
	if err := CheckRequirements(s.Require); err != nil {
		return nil, fmt.Errorf("skill %q: requirements not met: %w", skillName, err)
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

	// Load skill configuration (stored as credentials) and pass as creds.
	var creds map[string]string
	if len(s.Config) > 0 && r.configStore != nil {
		cfg, err := r.configStore.GetConfig(skillName)
		if err != nil {
			log.Printf("skill: warning: failed to load config for %q: %v", skillName, err)
		} else {
			creds = cfg
		}
	}

	// If the skill has a direct handler, use it.
	if s.Handler != nil {
		return s.Handler.Execute(ctx, inputs, creds)
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
