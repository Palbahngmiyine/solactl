package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/solapi/solactl/cmd"
	"github.com/solapi/solactl/pkg/apierror"
)

func main() {
	// Only fetch the CRM OpenAPI spec when the user is actually invoking a
	// `crm …` subcommand. Otherwise we'd add up to 30s of HTTP latency to
	// `--help`, `--version`, `configure`, `send`, etc.
	if invokesCRM(os.Args[1:]) {
		regCtx, regCancel := context.WithTimeout(context.Background(), 35*time.Second)
		cmd.RegisterDynamicCRM(regCtx)
		regCancel()
	}

	if err := cmd.Execute(); err != nil {
		classified := apierror.Classify(err)
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", classified.Message)
		if classified.Hint != "" {
			_, _ = fmt.Fprintf(os.Stderr, "Hint: %s\n", classified.Hint)
		}
		os.Exit(1)
	}
}

// invokesCRM returns true when the first non-flag positional argument is "crm".
// This mirrors the root persistent flags closely enough to avoid mistaking a
// flag value such as `--profile crm` for the command while still registering
// dynamic commands for common forms like `--profile prod crm records list`.
func invokesCRM(args []string) bool {
	skipNext := false
	for i, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--" {
			if i+1 >= len(args) {
				return false
			}
			return args[i+1] == "crm"
		}
		if strings.HasPrefix(a, "--") {
			name := strings.TrimPrefix(a, "--")
			if idx := strings.IndexByte(name, '='); idx >= 0 {
				name = name[:idx]
			}
			switch name {
			case "api-key", "api-secret", "profile", "timeout":
				if !strings.Contains(a, "=") {
					skipNext = true
				}
			}
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "crm"
	}
	return false
}
