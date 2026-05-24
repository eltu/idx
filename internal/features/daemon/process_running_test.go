package daemon

import (
	"os"
	"testing"
)

func TestProcessRunningReturnsFalseForZeroPID(t *testing.T) {
	if processRunning(0) {
		t.Fatal("expected false for PID 0")
	}
}

func TestProcessRunningReturnsFalseForNegativePID(t *testing.T) {
	if processRunning(-1) {
		t.Fatal("expected false for negative PID")
	}
}

func TestProcessRunningReturnsTrueForCurrentProcess(t *testing.T) {
	// The current process is guaranteed to be running.
	if !processRunning(os.Getpid()) {
		t.Fatal("expected current process to be running")
	}
}
