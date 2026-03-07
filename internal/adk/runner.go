package adk

import (
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"

	"github.com/neomody77/kuro/internal/skill"
)

// NewAgent creates an ADK LLMAgent backed by the given LLM and skill registry.
func NewAgent(llm model.LLM, registry *skill.Registry, systemPrompt string) (agent.Agent, error) {
	var tools []tool.Tool
	if registry != nil {
		tools = SkillToADKTools(registry)
	}

	return llmagent.New(llmagent.Config{
		Name:        "kuro",
		Description: "Kuro personal AI assistant",
		Model:       llm,
		Instruction: systemPrompt,
		Tools:       tools,
	})
}

// NewRunner creates an ADK Runner with the given session service.
func NewRunner(a agent.Agent, sessionSvc session.Service) (*runner.Runner, error) {
	return runner.New(runner.Config{
		AppName:        "kuro",
		Agent:          a,
		SessionService: sessionSvc,
	})
}
