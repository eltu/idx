package cli

import (
	"fmt"
	"strings"
)

func parseInspectArguments(arguments []string) (string, error) {
	if len(arguments) == 0 {
		return "", nil
	}

	if len(arguments) > 1 {
		return "", fmt.Errorf("inspect accepts at most one path: got %v, expected idx inspect [path]", arguments)
	}

	inspectPath := strings.TrimSpace(arguments[0])
	if inspectPath == "" || strings.HasPrefix(inspectPath, "--") {
		return "", fmt.Errorf("invalid inspect path %q: expected idx inspect [path]", arguments[0])
	}

	return inspectPath, nil
}
