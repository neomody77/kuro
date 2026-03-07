// Package shell implements shell command execution actions.
package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ExecAction runs a shell command and returns stdout, stderr, and exit code.
type ExecAction struct{}

func (a *ExecAction) Execute(ctx context.Context, params map[string]any, creds map[string]string) (any, error) {
	command, _ := params["command"].(string)
	if command == "" {
		return nil, fmt.Errorf("shell.exec: 'command' parameter is required")
	}

	timeoutF, _ := params["timeout"].(float64)
	timeout := time.Duration(timeoutF) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	dir, _ := params["dir"].(string)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return nil, fmt.Errorf("shell.exec: command timed out after %s", timeout)
		} else {
			return nil, fmt.Errorf("shell.exec: %w", err)
		}
	}

	return map[string]any{
		"stdout":    stdout.String(),
		"stderr":    stderr.String(),
		"exit_code": exitCode,
	}, nil
}
