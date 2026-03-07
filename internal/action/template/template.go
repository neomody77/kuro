// Package template implements template rendering actions.
package template

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// RenderAction renders a Go text/template with provided variables.
type RenderAction struct {
	DocumentsDir string // path to user's documents directory for loading templates
}

func (a *RenderAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	tmplStr, _ := params["template"].(string)
	tmplFile, _ := params["template_file"].(string)

	if tmplStr == "" && tmplFile == "" {
		return nil, fmt.Errorf("template.render: 'template' or 'template_file' parameter is required")
	}

	// Load template from file if specified.
	if tmplFile != "" && tmplStr == "" {
		content, err := a.loadTemplate(tmplFile)
		if err != nil {
			return nil, err
		}
		tmplStr = content
	}

	// Build template data from vars parameter or all params.
	vars, ok := params["vars"].(map[string]any)
	if !ok {
		vars = make(map[string]any)
		for k, v := range params {
			if k != "template" && k != "template_file" {
				vars[k] = v
			}
		}
	}

	t, err := template.New("render").Funcs(templateFuncs()).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("template.render: parse: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("template.render: execute: %w", err)
	}

	return map[string]any{
		"output": buf.String(),
	}, nil
}

func (a *RenderAction) loadTemplate(name string) (string, error) {
	if a.DocumentsDir == "" {
		return "", fmt.Errorf("template.render: documents directory not configured")
	}

	// Prevent path traversal.
	clean := filepath.Clean("/" + name)
	path := filepath.Join(a.DocumentsDir, clean)
	if !strings.HasPrefix(path, filepath.Clean(a.DocumentsDir)+string(filepath.Separator)) && path != filepath.Clean(a.DocumentsDir) {
		return "", fmt.Errorf("template.render: path %q escapes documents directory", name)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("template.render: load %q: %w", name, err)
	}
	return string(data), nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
		"trim":     strings.TrimSpace,
		"contains": strings.Contains,
		"replace":  strings.ReplaceAll,
		"join":     strings.Join,
		"split":    strings.Split,
		"default": func(def, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
	}
}
