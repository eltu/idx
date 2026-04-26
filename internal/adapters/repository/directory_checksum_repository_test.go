package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"idx/internal/core/ports"
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

	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
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

	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
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

func TestDirectoryChecksumRepositorySaveAndLoadSnapshotPreservesMetadata(t *testing.T) {
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()

	snapshot := ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{
		"a.go": {Checksum: "111", Size: 10, ModTimeUnixNano: 100},
		"b.go": {Checksum: "222", Size: 20, ModTimeUnixNano: 200},
	}}

	if err := repo.SaveSnapshot(dir, snapshot); err != nil {
		t.Fatalf("expected snapshot save to succeed, got %v", err)
	}

	loaded, exists, err := repo.LoadSnapshot(dir)
	if err != nil {
		t.Fatalf("expected snapshot load to succeed, got %v", err)
	}
	if !exists {
		t.Fatal("expected snapshot to exist")
	}

	if loaded.Files["a.go"].Size != 10 || loaded.Files["a.go"].ModTimeUnixNano != 100 {
		t.Fatalf("expected metadata for a.go to be preserved, got %+v", loaded.Files["a.go"])
	}
	if loaded.Files["b.go"].Checksum != "222" {
		t.Fatalf("expected checksum for b.go to be preserved, got %q", loaded.Files["b.go"].Checksum)
	}
}

func TestDirectoryChecksumRepositoryLoadSnapshotSupportsLegacyPayload(t *testing.T) {
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()

	legacy := checksumPayload{Files: map[string]string{"legacy.go": "abc"}}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("expected legacy payload marshal to succeed, got %v", err)
	}

	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
	if err := os.MkdirAll(filepath.Dir(checksumPath), 0750); err != nil {
		t.Fatalf("expected checksum dir creation to succeed, got %v", err)
	}
	if err := os.WriteFile(checksumPath, encoded, 0600); err != nil {
		t.Fatalf("expected legacy payload write to succeed, got %v", err)
	}

	snapshot, exists, err := repo.LoadSnapshot(dir)
	if err != nil {
		t.Fatalf("expected snapshot load to succeed for legacy payload, got %v", err)
	}
	if !exists {
		t.Fatal("expected snapshot to exist for legacy payload")
	}

	state := snapshot.Files["legacy.go"]
	if state.Checksum != "abc" {
		t.Fatalf("expected legacy checksum to be loaded, got %q", state.Checksum)
	}
	if state.Size != 0 || state.ModTimeUnixNano != 0 {
		t.Fatalf("expected empty metadata for legacy payload, got %+v", state)
	}
}

func TestCloneChecksumMapReturnsIndependentCopy(t *testing.T) {
	original := map[string]string{"a.go": "111"}
	cloned := cloneChecksumMap(original)

	if cloned["a.go"] != "111" {
		t.Fatalf("expected cloned value 111, got %q", cloned["a.go"])
	}

	original["a.go"] = "changed"
	if cloned["a.go"] != "111" {
		t.Fatalf("expected clone to remain immutable after source change, got %q", cloned["a.go"])
	}
}

func TestCloneChecksumMapNilInputReturnsEmptyMap(t *testing.T) {
	cloned := cloneChecksumMap(nil)
	if len(cloned) != 0 {
		t.Fatalf("expected empty map for nil input, got %d entries", len(cloned))
	}
}

func TestDirectoryChecksumRepositoryLoadAndSaveSnapshotInvalidPathErrors(t *testing.T) {
	repo := NewDirectoryChecksumRepository()

	if _, _, err := repo.LoadSnapshot("\x00invalid"); err == nil {
		t.Fatal("expected load snapshot error for invalid directory path")
	}

	err := repo.SaveSnapshot("\x00invalid", ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.go": {Checksum: "1"}}})
	if err == nil {
		t.Fatal("expected save snapshot error for invalid directory path")
	}
}

func TestPayloadToSnapshotPrefersFileStatesOverLegacyFiles(t *testing.T) {
	payload := checksumPayload{
		Files: map[string]string{"legacy.go": "legacy"},
		FileStates: map[string]checksumFileState{
			"modern.go": {Checksum: "modern", Size: 42, ModTimeUnixNano: 77},
		},
	}

	snapshot := payloadToSnapshot(payload)
	if len(snapshot.Files) != 1 {
		t.Fatalf("expected only fileStates to be used, got %d entries", len(snapshot.Files))
	}

	state := snapshot.Files["modern.go"]
	if state.Checksum != "modern" || state.Size != 42 || state.ModTimeUnixNano != 77 {
		t.Fatalf("unexpected snapshot state %+v", state)
	}
}
