package indexing

import (
	"testing"

	"idx/internal/core/domain"
)

func TestMergeInspectDocumentsSkipsNilStats(t *testing.T) {
	target := domain.NewInvertedIndex()
	source := domain.NewInvertedIndex()
	// Manually insert nil docStats to trigger the nil skip branch
	source.Documents["nilkey"] = nil
	source.Documents["validkey"] = &domain.DocStats{Name: "valid.go", Path: "valid.go", Length: 5}
	mergeInspectDocuments(target, "/repo", source)
	// Only "validkey" should be merged
	if _, ok := target.Documents["/repo::validkey"]; !ok {
		t.Fatal("expected /repo::validkey to be merged")
	}
	if _, ok := target.Documents["/repo::nilkey"]; ok {
		t.Fatal("expected nil key to be skipped")
	}
}
