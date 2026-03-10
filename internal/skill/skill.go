// Package skill manages the skill registry and built-in skills.
package skill

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neomody77/kuro/internal/action/ai"
	"github.com/neomody77/kuro/internal/action/email"
	fileaction "github.com/neomody77/kuro/internal/action/file"
	httpaction "github.com/neomody77/kuro/internal/action/http"
	"github.com/neomody77/kuro/internal/action/shell"
	"github.com/neomody77/kuro/internal/action/template"
	"github.com/neomody77/kuro/internal/action/transform"
	"github.com/neomody77/kuro/internal/credential"
	"github.com/neomody77/kuro/internal/document"
	"github.com/neomody77/kuro/internal/pipeline"
	"github.com/neomody77/kuro/internal/provider"
)

// CoreConfig holds the dependencies needed to create core skills.
type CoreConfig struct {
	CredentialStore *credential.Store
	DocumentStore   *document.Store
	WorkspaceDir    string
	DocumentsDir    string
	PipelinesDir    string
	Providers       *provider.Registry
}

// RegisterDefaults registers all built-in core skills into the given registry.
func RegisterDefaults(r *Registry, cfg CoreConfig) {
	// Credential skill (actions: list, get, save, update, delete)
	if cfg.CredentialStore != nil {
		r.Register(&Skill{
			Name:        "credential",
			Description: "Manage encrypted credentials in the vault. Actions: list, get, save, update, delete. All ops use 'id' as primary key.",
			Inputs: []SkillParam{
				{Name: "action", Required: true},
				{Name: "id"},
				{Name: "name"},
				{Name: "type"},
				{Name: "data"},
			},
			Handler:     &credentialSkill{store: cfg.CredentialStore},
			Destructive: true,
			Source:      "builtin",
		})
	}

	// Document skill (actions: list, get, save, rename, delete, search)
	if cfg.DocumentStore != nil {
		r.Register(&Skill{
			Name:        "document",
			Description: "Manage documents in the store. Actions: list, get, save, rename, delete, search",
			Inputs: []SkillParam{
				{Name: "action", Required: true},
				{Name: "path"},
				{Name: "new_path"},
				{Name: "content"},
				{Name: "query"},
			},
			Handler:     &documentSkill{store: cfg.DocumentStore},
			Destructive: true,
			Source:      "builtin",
		})
	}

	// Pipeline skill (actions: create, list, get, update, activate, deactivate, delete, execute)
	if cfg.PipelinesDir != "" {
		r.Register(&Skill{
			Name:        "pipeline",
			Description: "Manage workflows/pipelines. Actions: create, list, get, update, activate, deactivate, delete, execute",
			Inputs: []SkillParam{
				{Name: "action", Required: true},
				{Name: "name"},
				{Name: "id"},
				{Name: "active"},
				{Name: "nodes"},
				{Name: "connections"},
				{Name: "settings"},
			},
			Handler:     &pipelineSkill{dir: cfg.PipelinesDir},
			Destructive: true,
			Source:      "builtin",
		})
	}

	// Shell skill
	r.Register(&Skill{
		Name:        "shell",
		Description: "Run a shell command",
		Inputs:      []SkillParam{{Name: "command", Required: true}},
		Handler:     &shell.ExecAction{},
		Destructive: true,
		Source:      "builtin",
	})

	// Email skill (actions: fetch, send)
	r.Register(&Skill{
		Name:        "email",
		Description: "Email operations. Actions: fetch (IMAP), send (SMTP)",
		Inputs:      []SkillParam{{Name: "action", Required: true}},
		Handler:     &emailSkill{},
		Source:      "builtin",
	})

	// HTTP skill
	r.Register(&Skill{
		Name:        "http",
		Description: "Make an HTTP request",
		Inputs:      []SkillParam{{Name: "url", Required: true}},
		Handler:     &httpaction.RequestAction{},
		Source:      "builtin",
	})

	// File skill (actions: read, write, list, delete, rename)
	r.Register(&Skill{
		Name:        "file",
		Description: "File operations. Actions: read, write, list, delete, rename",
		Inputs: []SkillParam{
			{Name: "action", Required: true},
			{Name: "path"},
			{Name: "new_path"},
			{Name: "content"},
		},
		Handler:     &fileSkill{workspaceDir: cfg.WorkspaceDir},
		Destructive: true,
		Source:      "builtin",
	})

	// Transform skill
	r.Register(&Skill{
		Name:        "transform",
		Description: "JQ-like data transformation",
		Inputs:      []SkillParam{{Name: "expr", Required: true}},
		Handler:     &transform.JqAction{},
		Source:      "builtin",
	})

	// Template skill
	r.Register(&Skill{
		Name:        "template",
		Description: "Render a Go template",
		Handler:     &template.RenderAction{DocumentsDir: cfg.DocumentsDir},
		Source:      "builtin",
	})

	// AI skill
	r.Register(&Skill{
		Name:        "ai",
		Description: "Call an LLM for text completion",
		Handler:     &ai.CompleteAction{Providers: cfg.Providers},
		Source:      "builtin",
	})

	// Web Search skill (Tavily)
	r.Register(&Skill{
		Name:        "web_search",
		Description: "Search the web for real-time information using Tavily. Actions: search",
		Inputs: []SkillParam{
			{Name: "action", Required: true},
			{Name: "query", Required: true},
			{Name: "max_results", Type: "integer"},
		},
		Config: []SkillParam{
			{Name: "api_key", Required: true, Type: "password"},
		},
		Handler: &webSearchSkill{},
		Source:  "builtin",
	})
}

// --- Credential skill (unified) ---

type credentialSkill struct {
	store *credential.Store
}

func (s *credentialSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	switch action {
	case "list":
		list, err := s.store.List()
		if err != nil {
			return nil, fmt.Errorf("credential list: %w", err)
		}
		items := make([]map[string]any, len(list))
		for i, c := range list {
			items[i] = map[string]any{"id": c.ID, "name": c.Name, "type": c.Type}
		}
		return map[string]any{"credentials": items, "count": len(items)}, nil

	case "get":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("credential get: 'id' is required")
		}
		c, err := s.store.Get(id)
		if err != nil {
			return nil, fmt.Errorf("credential get: %w", err)
		}
		return map[string]any{"id": c.ID, "name": c.Name, "type": c.Type, "data": c.Data}, nil

	case "save":
		name, _ := params["name"].(string)
		typ, _ := params["type"].(string)
		if name == "" || typ == "" {
			return nil, fmt.Errorf("credential save: 'name' and 'type' are required")
		}
		data := parseDataParam(params)
		cred := credential.Credential{Name: name, Type: typ, Data: data}
		id, err := s.store.Save(cred)
		if err != nil {
			return nil, fmt.Errorf("credential save: %w", err)
		}
		return map[string]any{"ok": true, "id": id}, nil

	case "update":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("credential update: 'id' is required")
		}
		existing, err := s.store.Get(id)
		if err != nil {
			return nil, fmt.Errorf("credential update: %w", err)
		}
		if name, ok := params["name"].(string); ok && name != "" {
			existing.Name = name
		}
		if typ, ok := params["type"].(string); ok && typ != "" {
			existing.Type = typ
		}
		data := parseDataParam(params)
		for k, v := range data {
			existing.Data[k] = v
		}
		if _, err := s.store.Save(*existing); err != nil {
			return nil, fmt.Errorf("credential update: %w", err)
		}
		return map[string]any{"ok": true, "id": existing.ID}, nil

	case "delete":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("credential delete: 'id' is required")
		}
		if err := s.store.Delete(id); err != nil {
			return nil, fmt.Errorf("credential delete: %w", err)
		}
		return map[string]any{"ok": true}, nil

	default:
		return nil, fmt.Errorf("credential: unknown action %q (use: list, get, save, update, delete)", action)
	}
}

func parseDataParam(params map[string]any) map[string]string {
	data := make(map[string]string)
	if d, ok := params["data"].(map[string]any); ok {
		for k, v := range d {
			data[k] = fmt.Sprintf("%v", v)
		}
	}
	if d, ok := params["data"].(map[string]string); ok {
		data = d
	}
	return data
}

// --- Document skill (unified) ---

type documentSkill struct {
	store *document.Store
}

func (s *documentSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	switch action {
	case "list":
		path, _ := params["path"].(string)
		docs, err := s.store.List(path)
		if err != nil {
			return nil, fmt.Errorf("document list: %w", err)
		}
		items := make([]map[string]any, len(docs))
		for i, d := range docs {
			items[i] = map[string]any{"path": d.Path, "size": d.Size, "is_dir": d.IsDir}
		}
		return map[string]any{"documents": items, "count": len(items)}, nil

	case "get":
		path, _ := params["path"].(string)
		if path == "" {
			return nil, fmt.Errorf("document get: 'path' is required")
		}
		doc, err := s.store.Get(path)
		if err != nil {
			return nil, fmt.Errorf("document get: %w", err)
		}
		return map[string]any{"path": doc.Path, "content": doc.Content, "size": doc.Size}, nil

	case "save":
		path, _ := params["path"].(string)
		content, _ := params["content"].(string)
		if path == "" {
			return nil, fmt.Errorf("document save: 'path' is required")
		}
		if err := s.store.Put(path, content); err != nil {
			return nil, fmt.Errorf("document save: %w", err)
		}
		return map[string]any{"ok": true}, nil

	case "delete":
		path, _ := params["path"].(string)
		if path == "" {
			return nil, fmt.Errorf("document delete: 'path' is required")
		}
		if err := s.store.Delete(path); err != nil {
			return nil, fmt.Errorf("document delete: %w", err)
		}
		return map[string]any{"ok": true}, nil

	case "rename":
		path, _ := params["path"].(string)
		newPath, _ := params["new_path"].(string)
		if path == "" || newPath == "" {
			return nil, fmt.Errorf("document rename: 'path' and 'new_path' are required")
		}
		if err := s.store.Rename(path, newPath); err != nil {
			return nil, fmt.Errorf("document rename: %w", err)
		}
		return map[string]any{"ok": true}, nil

	case "search":
		query, _ := params["query"].(string)
		if query == "" {
			return nil, fmt.Errorf("document search: 'query' is required")
		}
		docs, err := s.store.Search(query)
		if err != nil {
			return nil, fmt.Errorf("document search: %w", err)
		}
		items := make([]map[string]any, len(docs))
		for i, d := range docs {
			items[i] = map[string]any{"path": d.Path, "size": d.Size}
		}
		return map[string]any{"results": items, "count": len(items)}, nil

	default:
		return nil, fmt.Errorf("document: unknown action %q (use: list, get, save, rename, delete, search)", action)
	}
}

// --- Email skill (unified) ---

type emailSkill struct{}

func (s *emailSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	switch action {
	case "fetch":
		return (&email.FetchAction{}).Execute(ctx, params, creds)
	case "send":
		return (&email.SendAction{}).Execute(ctx, params, creds)
	default:
		return nil, fmt.Errorf("email: unknown action %q (use: fetch, send)", action)
	}
}

// --- File skill (unified) ---

type fileSkill struct {
	workspaceDir string
}

func (s *fileSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	switch action {
	case "read":
		return (&fileaction.ReadAction{WorkspaceDir: s.workspaceDir}).Execute(ctx, params, creds)
	case "write":
		return (&fileaction.WriteAction{WorkspaceDir: s.workspaceDir}).Execute(ctx, params, creds)
	case "list":
		return (&fileaction.ListAction{WorkspaceDir: s.workspaceDir}).Execute(ctx, params, creds)
	case "delete":
		return (&fileaction.DeleteAction{WorkspaceDir: s.workspaceDir}).Execute(ctx, params, creds)
	case "rename":
		return (&fileaction.RenameAction{WorkspaceDir: s.workspaceDir}).Execute(ctx, params, creds)
	default:
		return nil, fmt.Errorf("file: unknown action %q (use: read, write, list, delete, rename)", action)
	}
}

// --- Pipeline skill (unified) ---

type pipelineSkill struct {
	dir string
}

func (s *pipelineSkill) store() *pipeline.WorkflowStore {
	return pipeline.NewWorkflowStore(s.dir)
}

func (s *pipelineSkill) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	action, _ := params["action"].(string)
	switch action {
	case "create":
		name, _ := params["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("pipeline create: 'name' is required")
		}
		w := &pipeline.Workflow{
			ID:   fmt.Sprintf("wf_%d", timeNowUnixMilli()),
			Name: name,
		}
		// Parse nodes from []any
		if rawNodes, ok := params["nodes"].([]any); ok {
			for _, rn := range rawNodes {
				nm, ok := rn.(map[string]any)
				if !ok {
					continue
				}
				node := pipeline.Node{
					Name:       strVal(nm, "name"),
					Type:       strVal(nm, "type"),
					Parameters: mapVal(nm, "parameters"),
				}
				if pos, ok := nm["position"].([]any); ok && len(pos) == 2 {
					node.Position = [2]float64{toFloat(pos[0]), toFloat(pos[1])}
				}
				if tv, ok := nm["typeVersion"]; ok {
					node.TypeVersion = toFloat(tv)
				}
				w.Nodes = append(w.Nodes, node)
			}
		}
		// Parse connections from map[string]any
		if rawConns, ok := params["connections"].(map[string]any); ok {
			w.Connections = make(map[string]pipeline.NodeConnection)
			for src, val := range rawConns {
				connMap, ok := val.(map[string]any)
				if !ok {
					continue
				}
				nc := pipeline.NodeConnection{}
				if mainRaw, ok := connMap["main"].([]any); ok {
					for _, outputGroup := range mainRaw {
						targets, ok := outputGroup.([]any)
						if !ok {
							continue
						}
						var cts []pipeline.ConnectionTarget
						for _, t := range targets {
							tm, ok := t.(map[string]any)
							if !ok {
								continue
							}
							cts = append(cts, pipeline.ConnectionTarget{
								Node:  strVal(tm, "node"),
								Type:  strVal(tm, "type"),
								Index: intVal(tm, "index"),
							})
						}
						nc.Main = append(nc.Main, cts)
					}
				}
				w.Connections[src] = nc
			}
		}
		// Parse settings
		if rawSettings, ok := params["settings"].(map[string]any); ok {
			if tz, ok := rawSettings["timezone"].(string); ok {
				w.Settings.Timezone = tz
			}
			if timeout, ok := rawSettings["executionTimeout"]; ok {
				w.Settings.ExecutionTimeout = intFromAny(timeout)
			}
		}
		if active, ok := params["active"].(bool); ok {
			w.Active = active
		}
		if err := s.store().Save(w); err != nil {
			return nil, fmt.Errorf("pipeline create: %w", err)
		}
		return map[string]any{"ok": true, "id": w.ID, "name": w.Name}, nil

	case "list":
		list, err := s.store().List()
		if err != nil {
			return nil, fmt.Errorf("pipeline list: %w", err)
		}
		items := make([]map[string]any, len(list))
		for i, w := range list {
			items[i] = map[string]any{
				"id": w.ID, "name": w.Name, "active": w.Active,
				"nodeCount": len(w.Nodes),
			}
		}
		return map[string]any{"pipelines": items, "count": len(items)}, nil

	case "get":
		id, _ := params["id"].(string)
		name, _ := params["name"].(string)
		if id == "" && name == "" {
			return nil, fmt.Errorf("pipeline get: 'id' or 'name' is required")
		}
		if id != "" {
			w, err := s.store().Get(id)
			if err != nil {
				return nil, fmt.Errorf("pipeline get: %w", err)
			}
			return w, nil
		}
		// Lookup by name
		list, err := s.store().List()
		if err != nil {
			return nil, fmt.Errorf("pipeline get: %w", err)
		}
		nameLower := strings.ToLower(name)
		for _, w := range list {
			if strings.ToLower(w.Name) == nameLower {
				return w, nil
			}
		}
		// Try substring match
		for _, w := range list {
			if strings.Contains(strings.ToLower(w.Name), nameLower) {
				return w, nil
			}
		}
		return nil, fmt.Errorf("pipeline get: no workflow found matching name %q", name)

	case "update":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("pipeline update: 'id' is required")
		}
		w, err := s.store().Get(id)
		if err != nil {
			return nil, fmt.Errorf("pipeline update: %w", err)
		}
		if name, ok := params["name"].(string); ok && name != "" {
			w.Name = name
		}
		if active, ok := params["active"].(bool); ok {
			w.Active = active
		}
		if rawNodes, ok := params["nodes"].([]any); ok {
			w.Nodes = nil
			for _, rn := range rawNodes {
				nm, ok := rn.(map[string]any)
				if !ok {
					continue
				}
				node := pipeline.Node{
					Name:       strVal(nm, "name"),
					Type:       strVal(nm, "type"),
					Parameters: mapVal(nm, "parameters"),
				}
				if pos, ok := nm["position"].([]any); ok && len(pos) == 2 {
					node.Position = [2]float64{toFloat(pos[0]), toFloat(pos[1])}
				}
				if tv, ok := nm["typeVersion"]; ok {
					node.TypeVersion = toFloat(tv)
				}
				w.Nodes = append(w.Nodes, node)
			}
		}
		if rawConns, ok := params["connections"].(map[string]any); ok {
			w.Connections = make(map[string]pipeline.NodeConnection)
			for src, val := range rawConns {
				connMap, ok := val.(map[string]any)
				if !ok {
					continue
				}
				nc := pipeline.NodeConnection{}
				if mainRaw, ok := connMap["main"].([]any); ok {
					for _, outputGroup := range mainRaw {
						targets, ok := outputGroup.([]any)
						if !ok {
							continue
						}
						var cts []pipeline.ConnectionTarget
						for _, t := range targets {
							tm, ok := t.(map[string]any)
							if !ok {
								continue
							}
							cts = append(cts, pipeline.ConnectionTarget{
								Node:  strVal(tm, "node"),
								Type:  strVal(tm, "type"),
								Index: intVal(tm, "index"),
							})
						}
						nc.Main = append(nc.Main, cts)
					}
				}
				w.Connections[src] = nc
			}
		}
		if rawSettings, ok := params["settings"].(map[string]any); ok {
			if tz, ok := rawSettings["timezone"].(string); ok {
				w.Settings.Timezone = tz
			}
			if timeout, ok := rawSettings["executionTimeout"]; ok {
				w.Settings.ExecutionTimeout = intFromAny(timeout)
			}
		}
		if err := s.store().Save(w); err != nil {
			return nil, fmt.Errorf("pipeline update: %w", err)
		}
		return map[string]any{"ok": true, "id": w.ID, "name": w.Name}, nil

	case "activate":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("pipeline activate: 'id' is required")
		}
		w, err := s.store().Get(id)
		if err != nil {
			return nil, fmt.Errorf("pipeline activate: %w", err)
		}
		w.Active = true
		if err := s.store().Save(w); err != nil {
			return nil, fmt.Errorf("pipeline activate: %w", err)
		}
		return map[string]any{"ok": true, "id": w.ID, "active": true}, nil

	case "deactivate":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("pipeline deactivate: 'id' is required")
		}
		w, err := s.store().Get(id)
		if err != nil {
			return nil, fmt.Errorf("pipeline deactivate: %w", err)
		}
		w.Active = false
		if err := s.store().Save(w); err != nil {
			return nil, fmt.Errorf("pipeline deactivate: %w", err)
		}
		return map[string]any{"ok": true, "id": w.ID, "active": false}, nil

	case "execute":
		id, _ := params["id"].(string)
		name, _ := params["name"].(string)
		if id == "" && name == "" {
			return nil, fmt.Errorf("pipeline execute: 'id' or 'name' is required")
		}
		var w *pipeline.Workflow
		if id != "" {
			got, getErr := s.store().Get(id)
			if getErr != nil {
				return nil, fmt.Errorf("pipeline execute: %w", getErr)
			}
			w = got
		} else {
			list, listErr := s.store().List()
			if listErr != nil {
				return nil, fmt.Errorf("pipeline execute: %w", listErr)
			}
			nameLower := strings.ToLower(name)
			for _, wf := range list {
				if strings.ToLower(wf.Name) == nameLower || strings.Contains(strings.ToLower(wf.Name), nameLower) {
					w = wf
					break
				}
			}
			if w == nil {
				return nil, fmt.Errorf("pipeline execute: no workflow found matching %q", name)
			}
		}
		return map[string]any{"ok": true, "id": w.ID, "name": w.Name, "note": "execution queued"}, nil

	case "delete":
		id, _ := params["id"].(string)
		if id == "" {
			return nil, fmt.Errorf("pipeline delete: 'id' is required")
		}
		if err := s.store().Delete(id); err != nil {
			return nil, fmt.Errorf("pipeline delete: %w", err)
		}
		return map[string]any{"ok": true}, nil

	default:
		return nil, fmt.Errorf("pipeline: unknown action %q (use: create, list, get, update, activate, deactivate, execute, delete)", action)
	}
}

// Helper functions for parsing JSON-like params

func timeNowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func mapVal(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func intVal(m map[string]any, key string) int {
	return intFromAny(m[key])
}

func intFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// Verify all skill types implement ActionHandler at compile time.
var (
	_ pipeline.ActionHandler = (*credentialSkill)(nil)
	_ pipeline.ActionHandler = (*documentSkill)(nil)
	_ pipeline.ActionHandler = (*emailSkill)(nil)
	_ pipeline.ActionHandler = (*fileSkill)(nil)
	_ pipeline.ActionHandler = (*pipelineSkill)(nil)
	_ pipeline.ActionHandler = (*webSearchSkill)(nil)
)
