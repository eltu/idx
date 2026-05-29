package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeInspectDocuments_SkipsNilStats(t *testing.T) {
	t.Parallel()

	// Arrange
	target := NewInvertedIndex()
	source := NewInvertedIndex()
	// Manually insert nil docStats to trigger the nil skip branch
	source.Documents["nilkey"] = nil
	source.Documents["validkey"] = &DocStats{Name: "valid.go", Path: "valid.go", Length: 5}

	// Act
	mergeInspectDocuments(target, "/repo", source)

	// Assert: only "validkey" should be merged
	_, hasValid := target.Documents["/repo::validkey"]
	require.True(t, hasValid, "expected /repo::validkey to be merged")
	assert.NotContains(t, target.Documents, "/repo::nilkey", "expected nil key to be skipped")
}
