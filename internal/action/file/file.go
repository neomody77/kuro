// Package file implements file read/write actions scoped to a workspace directory.
package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadAction reads file content from the workspace.
type ReadAction struct {
	WorkspaceDir string
}

func (a *ReadAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("file.read: 'path' parameter is required")
	}

	absPath, err := a.safePath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("file.read: %w", err)
	}

	return map[string]any{
		"content": string(data),
		"path":    path,
		"size":    len(data),
	}, nil
}

// WriteAction writes or appends to a file in the workspace.
type WriteAction struct {
	WorkspaceDir string
}

func (a *WriteAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("file.write: 'path' parameter is required")
	}

	data, _ := params["data"].(string)
	appendMode, _ := params["append"].(bool)

	absPath, err := a.safePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nil, fmt.Errorf("file.write: create dirs: %w", err)
	}

	flag := os.O_WRONLY | os.O_CREATE
	if appendMode {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(absPath, flag, 0o644)
	if err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}
	defer f.Close()

	n, err := f.WriteString(data)
	if err != nil {
		return nil, fmt.Errorf("file.write: %w", err)
	}

	return map[string]any{
		"written": n,
		"path":    path,
		"append":  appendMode,
	}, nil
}

// ListAction lists directory contents in the workspace.
type ListAction struct {
	WorkspaceDir string
}

func (a *ListAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	path, _ := params["path"].(string)
	if path == "" {
		path = "."
	}

	absPath, err := a.safePath(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("file.list: %w", err)
	}

	var items []map[string]any
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, map[string]any{
			"name":  entry.Name(),
			"is_dir": entry.IsDir(),
			"size":  info.Size(),
		})
	}

	return map[string]any{
		"path":  path,
		"items": items,
		"count": len(items),
	}, nil
}

// DeleteAction deletes a file in the workspace.
type DeleteAction struct {
	WorkspaceDir string
}

func (a *DeleteAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	path, _ := params["path"].(string)
	if path == "" {
		return nil, fmt.Errorf("file.delete: 'path' parameter is required")
	}
	absPath, err := safePath(a.WorkspaceDir, path)
	if err != nil {
		return nil, err
	}
	if err := os.Remove(absPath); err != nil {
		return nil, fmt.Errorf("file.delete: %w", err)
	}
	return map[string]any{"ok": true, "path": path}, nil
}

// RenameAction renames/moves a file in the workspace.
type RenameAction struct {
	WorkspaceDir string
}

func (a *RenameAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	path, _ := params["path"].(string)
	newPath, _ := params["new_path"].(string)
	if path == "" || newPath == "" {
		return nil, fmt.Errorf("file.rename: 'path' and 'new_path' are required")
	}
	absOld, err := safePath(a.WorkspaceDir, path)
	if err != nil {
		return nil, err
	}
	absNew, err := safePath(a.WorkspaceDir, newPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(absNew), 0o755); err != nil {
		return nil, fmt.Errorf("file.rename: mkdir: %w", err)
	}
	if err := os.Rename(absOld, absNew); err != nil {
		return nil, fmt.Errorf("file.rename: %w", err)
	}
	return map[string]any{"ok": true, "from": path, "to": newPath}, nil
}

// safePath resolves a path within the workspace, preventing traversal.
func (a *ReadAction) safePath(rel string) (string, error) {
	return safePath(a.WorkspaceDir, rel)
}

func (a *WriteAction) safePath(rel string) (string, error) {
	return safePath(a.WorkspaceDir, rel)
}

func (a *ListAction) safePath(rel string) (string, error) {
	return safePath(a.WorkspaceDir, rel)
}

func safePath(base, rel string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("file: workspace directory not configured")
	}
	abs := filepath.Join(base, filepath.Clean("/"+rel))
	absClean := filepath.Clean(abs)
	baseClean := filepath.Clean(base)
	if !strings.HasPrefix(absClean, baseClean+string(filepath.Separator)) && absClean != baseClean {
		return "", fmt.Errorf("file: path %q escapes workspace directory", rel)
	}
	return absClean, nil
}
