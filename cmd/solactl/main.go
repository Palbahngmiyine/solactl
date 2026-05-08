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
// Flag values (`--profile foo`, `-d`) are skipped without trying to map every
// flag's arity — a flag-shaped token never falls back to "crm subcommand", so
// over-skipping is safe here.
func invokesCRM(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		return a == "crm"
	}
	return false
}
