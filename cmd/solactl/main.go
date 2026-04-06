package main

import (
	"fmt"
	"os"

	"github.com/solapi/solactl/cmd"
	"github.com/solapi/solactl/pkg/apierror"
)

func main() {
	if err := cmd.Execute(); err != nil {
		classified := apierror.Classify(err)
		_, _ = fmt.Fprintf(os.Stderr, "Error: %s\n", classified.Message)
		if classified.Hint != "" {
			_, _ = fmt.Fprintf(os.Stderr, "Hint: %s\n", classified.Hint)
		}
		os.Exit(1)
	}
}
