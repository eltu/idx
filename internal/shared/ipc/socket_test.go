package ipc

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSocketPathReturnsSockSuffix(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.HasSuffix(path, ".sock") {
		t.Errorf("expected .sock suffix, got %q", path)
	}
}

func TestSocketPathContainsProjectName(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.Contains(filepath.Base(path), "myproject") {
		t.Errorf("expected project name in socket path, got %q", path)
	}
}

func TestSocketPathContainsDotIdxDir(t *testing.T) {
	path := SocketPath("/home/user/myproject")
	if !strings.Contains(path, ".idx") {
		t.Errorf("expected .idx dir in socket path, got %q", path)
	}
}

func TestSanitizeSocketSegmentPreservesSafeChars(t *testing.T) {
	got := sanitizeSocketSegment("my-project_v2.go")
	if got != "my-project_v2.go" {
		t.Errorf("expected unchanged name, got %q", got)
	}
}

func TestSanitizeSocketSegmentReplacesUnsafeChars(t *testing.T) {
	got := sanitizeSocketSegment("my project@2026")
	if got != "my_project_2026" {
		t.Errorf("expected %q, got %q", "my_project_2026", got)
	}
}

func TestSanitizeSocketSegmentEmptyNameFallback(t *testing.T) {
	got := sanitizeSocketSegment("")
	if got != unknownProject {
		t.Errorf("expected %q for empty name, got %q", unknownProject, got)
	}
}

func TestSanitizeSocketSegmentDotNameFallback(t *testing.T) {
	got := sanitizeSocketSegment(".")
	if got != unknownProject {
		t.Errorf("expected %q for dot name, got %q", unknownProject, got)
	}
}

func TestSanitizeSocketSegmentSeparatorFallback(t *testing.T) {
	got := sanitizeSocketSegment(string(filepath.Separator))
	if got != unknownProject {
		t.Errorf("expected %q for separator, got %q", unknownProject, got)
	}
}

func TestSanitizeSocketSegmentAllUnsafeCharsFallback(t *testing.T) {
	got := sanitizeSocketSegment("@@@")
	if got != unknownProject {
		t.Errorf("expected %q for all-unsafe name, got %q", unknownProject, got)
	}
}

func TestSanitizeSocketSegmentTrimsLeadingAndTrailingDots(t *testing.T) {
	got := sanitizeSocketSegment("...abc...")
	if strings.HasPrefix(got, ".") || strings.HasSuffix(got, ".") {
		t.Errorf("expected no leading/trailing dots, got %q", got)
	}
}

func TestIsSocketSafeCharAcceptsLettersDigitsAndSpecials(t *testing.T) {
	safe := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	for i := range len(safe) {
		if !isSocketSafeChar(safe[i]) {
			t.Errorf("expected safe char %q to be accepted", safe[i])
		}
	}
}

func TestIsSocketSafeCharRejectsSpaceAndAt(t *testing.T) {
	for _, ch := range []byte(" @#!") {
		if isSocketSafeChar(ch) {
			t.Errorf("expected unsafe char %q to be rejected", ch)
		}
	}
}
