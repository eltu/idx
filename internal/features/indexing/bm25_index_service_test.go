package indexing_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestBM25IndexService_BuildIndex_BuildsDocumentsTermsAndPathMetadata(t *testing.T) {
	t.Parallel()

	// Arrange
	service := indexing.NewBM25IndexService()
	docs := []indexing.IndexDocument{
		{Name: "a.txt", Path: filepath.Join("repo", "a.txt"), Content: "Go go idx"},
		{Name: "b.txt", Path: filepath.Join("repo", "sub", "b.txt"), Content: "idx search"},
	}

	// Act
	index, err := service.BuildIndex(docs)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, index.DocumentCount)
	require.NotNil(t, index.Terms["go"])
	assert.Equal(t, 2, index.Terms["go"].Docs["a.txt"].TF)
	require.NotNil(t, index.Terms["idx"])
	assert.Len(t, index.Terms["idx"].Docs, 2)
	assert.True(t, index.PathTerms["repo"]["a.txt"])
	assert.True(t, index.PathTerms["a.txt"]["a.txt"])
	assert.True(t, index.PathTerms["sub"]["b.txt"])
	assert.True(t, index.PathTerms["b.txt"]["b.txt"])
	assert.True(t, index.ExtensionTerms["txt"]["a.txt"])
	assert.True(t, index.ExtensionTerms["txt"]["b.txt"])
}

func TestBM25IndexService_BuildIndex_WithEmptyDocuments_ReturnsEmptyIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	service := indexing.NewBM25IndexService()

	// Act
	index, err := service.BuildIndex([]indexing.IndexDocument{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, index.DocumentCount)
	assert.Empty(t, index.Terms)
}
