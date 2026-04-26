package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryChecksumRepositoryLoadUsesInMemoryCacheWhenFileUnchanged(t *testing.T) {
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	checksums := map[string]string{"a.go": "111", "b.go": "222"}

	if err := repo.Save(dir, checksums); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	firstLoad, exists, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected first load to succeed, got %v", err)
	}
	if !exists {
		t.Fatal("expected checksum file to exist")
	}

	if len(firstLoad) != 2 {
		t.Fatalf("expected two checksums, got %d", len(firstLoad))
	}

	firstLoad["a.go"] = "changed-locally"
	secondLoad, exists, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected second load to succeed, got %v", err)
	}
	if !exists {
		t.Fatal("expected checksum file to exist on second load")
	}
	if secondLoad["a.go"] != "111" {
		t.Fatalf("expected cached checksum to remain immutable, got %q", secondLoad["a.go"])
	}
}

func TestDirectoryChecksumRepositoryLoadReloadsWhenDiskChecksumChanges(t *testing.T) {
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	initial := map[string]string{"a.go": "111"}

	if err := repo.Save(dir, initial); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	_, _, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected initial load to succeed, got %v", err)
	}

	updated := checksumPayload{Files: map[string]string{"a.go": "999", "c.go": "333"}}
	content, err := json.Marshal(updated)
	if err != nil {
		t.Fatalf("expected payload marshal to succeed, got %v", err)
	}

	checksumPath := filepath.Join(dir, ".idx", "checksum")
	if err := os.WriteFile(checksumPath, content, 0600); err != nil {
		t.Fatalf("expected external write to succeed, got %v", err)
	}

	reloaded, exists, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected reload to succeed, got %v", err)
	}
	if !exists {
		t.Fatal("expected checksum file to exist after external write")
	}

	if reloaded["a.go"] != "999" {
		t.Fatalf("expected updated checksum from disk, got %q", reloaded["a.go"])
	}
	if reloaded["c.go"] != "333" {
		t.Fatalf("expected new checksum from disk, got %q", reloaded["c.go"])
	}
}

func TestDirectoryChecksumRepositoryLoadClearsCacheWhenFileRemoved(t *testing.T) {
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()

	if err := repo.Save(dir, map[string]string{"a.go": "111"}); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	if _, _, err := repo.Load(dir); err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}

	checksumPath := filepath.Join(dir, ".idx", "checksum")
	if err := os.Remove(checksumPath); err != nil {
		t.Fatalf("expected checksum removal to succeed, got %v", err)
	}

	loaded, exists, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected load without file to succeed, got %v", err)
	}
	if exists {
		t.Fatal("expected checksum file to be absent")
	}
	if len(loaded) != 0 {
		t.Fatalf("expected empty checksum map when file is absent, got %d entries", len(loaded))
	}
}
