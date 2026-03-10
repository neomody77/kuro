package skill

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CheckRequirements validates that all runtime prerequisites are met.
// Returns nil if req is nil or all checks pass.
func CheckRequirements(req *SkillRequire) error {
	if req == nil {
		return nil
	}

	var errs []string

	for _, env := range req.Env {
		if os.Getenv(env) == "" {
			errs = append(errs, fmt.Sprintf("env %q not set", env))
		}
	}

	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			errs = append(errs, fmt.Sprintf("binary %q not found in PATH", bin))
		}
	}

	if len(req.OS) > 0 {
		found := false
		for _, o := range req.OS {
			if strings.EqualFold(o, runtime.GOOS) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Sprintf("OS %q not in allowed list %v", runtime.GOOS, req.OS))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}
