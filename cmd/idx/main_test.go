package main

import (
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestRunReturnsErrorForUnsupportedCommand(t *testing.T) {
	err := run([]string{"idx", "unknown-command"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected unsupported command error, got nil")
	}
}

func TestMainCallsExitWithCodeOneWhenRunFails(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitProcess
	os.Args = []string{"idx", "unknown-command"}
	t.Cleanup(func() {
		os.Args = originalArgs
		exitProcess = originalExit
	})

	exitCalled := false
	exitCode := 0
	exitProcess = func(code int) {
		exitCalled = true
		exitCode = code
		panic(errors.New("exit called"))
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic from exit hook, got nil")
		}
		if !exitCalled {
			t.Fatal("expected exit hook to be called")
		}
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	}()

	main()
}
