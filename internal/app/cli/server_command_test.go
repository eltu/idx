package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// --- findProjectRoot ---

func TestFindProjectRootFromRootItself(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".idx"), 0o750); err != nil {
		t.Fatal(err)
	}
	got, err := findProjectRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("expected %q, got %q", dir, got)
	}
}

func TestFindProjectRootFromSubdirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "core", "pkg")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".idx"), 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := findProjectRoot(nested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("expected root %q, got %q", root, got)
	}
}

func TestFindProjectRootReturnsErrorWhenNoDotIdx(t *testing.T) {
	dir := t.TempDir() // no .idx directory
	_, err := findProjectRoot(dir)
	if err == nil {
		t.Fatal("expected error when no .idx found, got nil")
	}
}

func TestFindProjectRootPrefersDeeperMarker(t *testing.T) {
	// Two nested .idx directories — should stop at the closest ancestor.
	outer := t.TempDir()
	inner := filepath.Join(outer, "sub")
	if err := os.MkdirAll(filepath.Join(inner, ".idx"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outer, ".idx"), 0o750); err != nil {
		t.Fatal(err)
	}

	got, err := findProjectRoot(inner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != inner {
		t.Errorf("expected inner root %q, got %q", inner, got)
	}
}
