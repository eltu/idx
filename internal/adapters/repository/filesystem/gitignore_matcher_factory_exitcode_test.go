package filesystem

import (
	"errors"
	"os/exec"
	"testing"
)

func TestExitCodeReturnsMinusOneForNonExitError(t *testing.T) {
	if exitCode(errors.New("plain error")) != -1 {
		t.Fatalf("expected -1 for non-exit error, got %d", exitCode(errors.New("plain error")))
	}
}

func TestExitCodeReturnsCommandExitStatus(t *testing.T) {
	command := exec.Command("git", "not-a-real-subcommand") //nolint:gosec
	err := command.Run()
	if err == nil {
		t.Fatal("expected git command to fail, got nil")
	}

	code := exitCode(err)
	if code <= 0 {
		t.Fatalf("expected positive exit code, got %d", code)
	}
}
