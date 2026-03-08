// Package adk provides adapters for integrating Google ADK with OpenAI-compatible APIs.
package adk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"log"
	"net/http"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// OpenAILLM implements model.LLM for OpenAI-compatible APIs (OpenAI, OpenRouter, etc).
type OpenAILLM struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAILLM(baseURL, apiKey, modelName string) *OpenAILLM {
	return &OpenAILLM{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   modelName,
		client:  http.DefaultClient,
	}
}

func (o *OpenAILLM) Name() string { return o.model }

func (o *OpenAILLM) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		oaiReq := convertRequest(req, o.model, stream)

		body, err := json.Marshal(oaiReq)
		if err != nil {
			yield(nil, fmt.Errorf("adk/openai: marshal request: %w", err))
			return
		}

		// Debug: log the messages being sent
		for i, msg := range oaiReq.Messages {
			tcCount := len(msg.ToolCalls)
			log.Printf("[adk/openai] msg[%d] role=%s content_len=%d tool_calls=%d tool_call_id=%q",
				i, msg.Role, len(msg.Content), tcCount, msg.ToolCallID)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			yield(nil, fmt.Errorf("adk/openai: create request: %w", err))
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

		resp, err := o.client.Do(httpReq)
		if err != nil {
			yield(nil, fmt.Errorf("adk/openai: http request: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			yield(nil, fmt.Errorf("adk/openai: status %d: %s", resp.StatusCode, truncate(string(respBody), 500)))
			return
		}

		if stream {
			processSSE(resp.Body, yield)
		} else {
			processNonStream(resp.Body, yield)
		}
	}
}

// --- OpenAI request types ---

type oaiRequest struct {
	Model    string       `json:"model"`
	Messages []oaiMessage `json:"messages"`
	Tools    []oaiTool    `json:"tools,omitempty"`
	Stream   bool         `json:"stream"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiFunction `json:"function"`
}

type oaiFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type oaiToolCall struct {
	Index    *int            `json:"index,omitempty"` // present in streaming deltas
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// --- OpenAI response types ---

type oaiResponse struct {
	Choices []oaiChoice `json:"choices"`
	Model   string      `json:"model"`
	Usage   *oaiUsage   `json:"usage,omitempty"`
}

type oaiChoice struct {
	Message      *oaiRespMessage `json:"message,omitempty"`
	Delta        *oaiRespMessage `json:"delta,omitempty"`
	FinishReason *string         `json:"finish_reason,omitempty"`
}

type oaiRespMessage struct {
	Role      string        `json:"role,omitempty"`
	Content   string        `json:"content,omitempty"`
	ToolCalls []oaiToolCall `json:"tool_calls,omitempty"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// --- Conversion: ADK → OpenAI ---

func convertRequest(req *model.LLMRequest, modelName string, stream bool) *oaiRequest {
	oai := &oaiRequest{
		Model:  modelName,
		Stream: stream,
	}

	// System instruction → system message
	if req.Config != nil && req.Config.SystemInstruction != nil {
		text := partsToText(req.Config.SystemInstruction.Parts)
		if text != "" {
			oai.Messages = append(oai.Messages, oaiMessage{Role: "system", Content: text})
		}
	}

	// Contents → messages
	for _, content := range req.Contents {
		msgs := contentToMessages(content)
		oai.Messages = append(oai.Messages, msgs...)
	}

	// Tools → tools
	if req.Config != nil {
		for _, tool := range req.Config.Tools {
			if tool == nil {
				continue
			}
			for _, fd := range tool.FunctionDeclarations {
				oai.Tools = append(oai.Tools, funcDeclToOAI(fd))
			}
		}
	}

	return oai
}

func contentToMessages(c *genai.Content) []oaiMessage {
	if c == nil {
		return nil
	}

	role := c.Role
	if role == "model" {
		role = "assistant"
	}

	// Separate text parts, function calls, and function responses.
	var textParts []string
	var toolCalls []oaiToolCall
	var toolResponses []oaiMessage

	for _, p := range c.Parts {
		switch {
		case p.FunctionCall != nil:
			fc := p.FunctionCall
			// Skip ADK-internal confirmation requests — not a real LLM tool call
			if fc.Name == toolconfirmation.FunctionCallName {
				continue
			}
			argsJSON, _ := json.Marshal(fc.Args)
			toolCalls = append(toolCalls, oaiToolCall{
				ID:   fc.ID,
				Type: "function",
				Function: oaiFunctionCall{
					Name:      fc.Name,
					Arguments: string(argsJSON),
				},
			})
		case p.FunctionResponse != nil:
			fr := p.FunctionResponse
			// Skip ADK-internal confirmation responses
			if fr.Name == toolconfirmation.FunctionCallName {
				continue
			}
			respJSON, _ := json.Marshal(fr.Response)
			toolResponses = append(toolResponses, oaiMessage{
				Role:       "tool",
				Content:    string(respJSON),
				ToolCallID: fr.ID,
			})
		case p.Text != "" && !p.Thought:
			textParts = append(textParts, p.Text)
		}
	}

	var msgs []oaiMessage

	// Assistant message with text and/or tool_calls
	if len(textParts) > 0 || len(toolCalls) > 0 {
		msg := oaiMessage{Role: role}
		if len(textParts) > 0 {
			msg.Content = strings.Join(textParts, "")
		}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		msgs = append(msgs, msg)
	}

	// Tool response messages (role: "tool")
	msgs = append(msgs, toolResponses...)

	// If nothing was extracted but we have a role, send empty content
	if len(msgs) == 0 && role != "" {
		msgs = append(msgs, oaiMessage{Role: role})
	}

	return msgs
}

func funcDeclToOAI(fd *genai.FunctionDeclaration) oaiTool {
	var params any
	if fd.Parameters != nil {
		params = schemaToJSON(fd.Parameters)
	} else if fd.ParametersJsonSchema != nil {
		params = fd.ParametersJsonSchema
	}
	return oaiTool{
		Type: "function",
		Function: oaiFunction{
			Name:        fd.Name,
			Description: fd.Description,
			Parameters:  params,
		},
	}
}

func schemaToJSON(s *genai.Schema) map[string]any {
	m := map[string]any{}
	if s.Type != "" {
		m["type"] = strings.ToLower(string(s.Type))
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Items != nil {
		m["items"] = schemaToJSON(s.Items)
	}
	if len(s.Properties) > 0 {
		props := map[string]any{}
		for k, v := range s.Properties {
			props[k] = schemaToJSON(v)
		}
		m["properties"] = props
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	return m
}

func partsToText(parts []*genai.Part) string {
	var sb strings.Builder
	for _, p := range parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// --- Conversion: OpenAI → ADK ---

func processNonStream(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	data, err := io.ReadAll(body)
	if err != nil {
		yield(nil, fmt.Errorf("adk/openai: read response: %w", err))
		return
	}

	var resp oaiResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		yield(nil, fmt.Errorf("adk/openai: parse response: %w (body: %s)", err, truncate(string(data), 200)))
		return
	}

	if len(resp.Choices) == 0 {
		yield(nil, fmt.Errorf("adk/openai: no choices in response"))
		return
	}

	choice := resp.Choices[0]
	msg := choice.Message
	if msg == nil {
		yield(nil, fmt.Errorf("adk/openai: nil message in choice"))
		return
	}

	llmResp := &model.LLMResponse{
		Content:      messageToContent(msg),
		TurnComplete: true,
		FinishReason: mapFinishReason(choice.FinishReason),
	}
	if resp.Usage != nil {
		llmResp.UsageMetadata = &genai.GenerateContentResponseUsageMetadata{
			PromptTokenCount:     int32(resp.Usage.PromptTokens),
			CandidatesTokenCount: int32(resp.Usage.CompletionTokens),
			TotalTokenCount:      int32(resp.Usage.TotalTokens),
		}
	}

	yield(llmResp, nil)
}

type toolCallAcc struct {
	id       string
	name     string
	argsJSON strings.Builder
}

func processSSE(body io.Reader, yield func(*model.LLMResponse, error) bool) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var toolCalls []toolCallAcc
	var textAcc strings.Builder // accumulate full text for TurnComplete

	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]
		if data == "[DONE]" {
			// Emit final TurnComplete with accumulated content
			content := &genai.Content{Role: "model"}
			if textAcc.Len() > 0 {
				content.Parts = append(content.Parts, genai.NewPartFromText(textAcc.String()))
			}
			if len(toolCalls) > 0 {
				for _, tc := range toolCalls {
					var args map[string]any
					json.Unmarshal([]byte(tc.argsJSON.String()), &args)
					content.Parts = append(content.Parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							ID:   tc.id,
							Name: tc.name,
							Args: args,
						},
					})
				}
			}
			if len(content.Parts) > 0 {
				yield(&model.LLMResponse{
					Content:      content,
					TurnComplete: true,
					FinishReason: genai.FinishReasonStop,
				}, nil)
			}
			return
		}

		var chunk oaiResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta
		if delta == nil {
			continue
		}

		// Text delta
		if delta.Content != "" {
			textAcc.WriteString(delta.Content)
			if !yield(&model.LLMResponse{
				Content: &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{genai.NewPartFromText(delta.Content)},
				},
				Partial: true,
			}, nil) {
				return
			}
		}

		// Tool call deltas — OpenAI sends index-based chunks
		for _, tc := range delta.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			// Grow slice to fit index
			for len(toolCalls) <= idx {
				toolCalls = append(toolCalls, toolCallAcc{})
			}
			if tc.ID != "" {
				toolCalls[idx].id = tc.ID
			}
			if tc.Function.Name != "" {
				toolCalls[idx].name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				toolCalls[idx].argsJSON.WriteString(tc.Function.Arguments)
			}
		}

		// Check finish reason for non-DONE termination
		if choice.FinishReason != nil && *choice.FinishReason == "stop" {
			// Will be handled by [DONE]
		}
	}

	if err := scanner.Err(); err != nil {
		yield(nil, fmt.Errorf("adk/openai: sse read error: %w", err))
	}
}


func messageToContent(msg *oaiRespMessage) *genai.Content {
	content := &genai.Content{Role: "model"}

	if msg.Content != "" {
		content.Parts = append(content.Parts, genai.NewPartFromText(msg.Content))
	}

	for _, tc := range msg.ToolCalls {
		var args map[string]any
		json.Unmarshal([]byte(tc.Function.Arguments), &args)
		content.Parts = append(content.Parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			},
		})
	}

	return content
}

func mapFinishReason(fr *string) genai.FinishReason {
	if fr == nil {
		return genai.FinishReasonUnspecified
	}
	switch *fr {
	case "stop":
		return genai.FinishReasonStop
	case "length":
		return genai.FinishReasonMaxTokens
	case "tool_calls":
		return genai.FinishReasonStop // ADK treats tool_calls as needing another turn
	case "content_filter":
		return genai.FinishReasonSafety
	default:
		return genai.FinishReasonOther
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
