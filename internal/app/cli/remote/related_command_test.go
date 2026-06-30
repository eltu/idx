package remote

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	featrelated "idx/internal/features/related"
	idxipc "idx/internal/shared/ipc"
)

// --- NewRemoteRelatedCommand ---

func TestNewRemoteRelatedCommand_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	c := NewRemoteRelatedCommand(NewSocketClient("/tmp/test.sock"), &fakeOutput{})

	require.NotNil(t, c)
}

// --- Run ---

func TestRemoteRelatedCommand_Run_TextFormat_WritesResults(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.RelatedResponse{
			Count: 2,
			Results: []idxipc.RelatedResult{
				{Path: "internal/features/search/service.go", Score: 0.85, Reason: "git"},
				{Path: "internal/features/search/port.go", Score: 0.60, Reason: "co-read"},
			},
		}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteRelatedCommand(NewSocketClient(sock), out)
	err := cmd.Run("internal/features/related/service.go", featrelated.Options{Format: featrelated.OutputText})

	// Assert
	require.NoError(t, err)
	assert.Len(t, out.lines, 2)
	assert.Contains(t, out.lines[0], "internal/features/search/service.go")
	assert.Contains(t, out.lines[1], "internal/features/search/port.go")
}

func TestRemoteRelatedCommand_Run_EmptyResults_WritesNoRelatedMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.RelatedResponse{Count: 0, Results: []idxipc.RelatedResult{}}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteRelatedCommand(NewSocketClient(sock), out)
	err := cmd.Run("some/file.go", featrelated.Options{Format: featrelated.OutputText})

	// Assert
	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Equal(t, msgNoRelatedFound, out.lines[0])
}

func TestRemoteRelatedCommand_Run_JSONFormat_WritesJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.RelatedResponse{
			Count:   1,
			Results: []idxipc.RelatedResult{{Path: "pkg/foo.go", Score: 0.9, Reason: "git"}},
		}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteRelatedCommand(NewSocketClient(sock), out)
	err := cmd.Run("main.go", featrelated.Options{Format: featrelated.OutputJSON})

	// Assert
	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Contains(t, out.lines[0], "pkg/foo.go")
}

func TestRemoteRelatedCommand_Run_CompactMode_WritesPathsOnly(t *testing.T) {
	t.Parallel()

	// Arrange
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.RelatedResponse{
			Count:   1,
			Results: []idxipc.RelatedResult{{Path: "internal/service.go", Score: 0.7, Reason: "term-overlap"}},
		}
	})
	out := &fakeOutput{}

	// Act
	cmd := NewRemoteRelatedCommand(NewSocketClient(sock), out)
	err := cmd.Run("main.go", featrelated.Options{Format: featrelated.OutputText, Compact: true})

	// Assert
	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Equal(t, "internal/service.go", out.lines[0])
}

func TestRemoteRelatedCommand_Run_ServerNotReachable_ReturnsError(t *testing.T) {
	t.Parallel()

	cmd := NewRemoteRelatedCommand(NewSocketClient("/tmp/nonexistent-idx-related-99.sock"), &fakeOutput{})
	err := cmd.Run("file.go", featrelated.Options{})

	require.Error(t, err)
}

func TestRemoteRelatedCommand_Run_WriteError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange — server returns one result, but the output writer always fails
	sock := fakeJSONRPCServer(t, func(_ string, _ []byte) any {
		return idxipc.RelatedResponse{
			Count:   1,
			Results: []idxipc.RelatedResult{{Path: "pkg/a.go", Score: 0.5, Reason: "git"}},
		}
	})

	// Act
	cmd := NewRemoteRelatedCommand(NewSocketClient(sock), &errorOutput{})
	err := cmd.Run("main.go", featrelated.Options{Format: featrelated.OutputText})

	// Assert
	require.Error(t, err)
}

// --- writeRelatedRespJSON ---

func TestWriteRelatedRespJSON_EmptySlice_WritesEmptyArray(t *testing.T) {
	t.Parallel()

	out := &fakeOutput{}
	err := writeRelatedRespJSON([]idxipc.RelatedResult{}, out)

	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Equal(t, "[]", out.lines[0])
}

func TestWriteRelatedRespJSON_WithResults_WritesJSONArray(t *testing.T) {
	t.Parallel()

	results := []idxipc.RelatedResult{
		{Path: "a.go", Score: 0.8, Reason: "git"},
		{Path: "b.go", Score: 0.5, Reason: "co-read"},
	}
	out := &fakeOutput{}
	err := writeRelatedRespJSON(results, out)

	require.NoError(t, err)
	require.Len(t, out.lines, 1)
	assert.Contains(t, out.lines[0], "a.go")
	assert.Contains(t, out.lines[0], "b.go")
}

// --- formatRelatedResult ---

func TestFormatRelatedResult_Compact_ReturnsPathOnly(t *testing.T) {
	t.Parallel()

	res := idxipc.RelatedResult{Path: "internal/features/search/service.go", Score: 0.95, Reason: "git"}
	line := formatRelatedResult(res, featrelated.Options{Compact: true})

	assert.Equal(t, "internal/features/search/service.go", line)
}

func TestFormatRelatedResult_NotCompact_IncludesScoreAndReason(t *testing.T) {
	t.Parallel()

	res := idxipc.RelatedResult{Path: "internal/features/search/service.go", Score: 0.95, Reason: "git"}
	line := formatRelatedResult(res, featrelated.Options{Compact: false})

	assert.Contains(t, line, "internal/features/search/service.go")
	assert.Contains(t, line, "git")
	assert.Contains(t, line, "0.95")
}

func TestFormatRelatedResult_NotCompact_IncludesParentheses(t *testing.T) {
	t.Parallel()

	res := idxipc.RelatedResult{Path: "pkg/foo.go", Score: 0.5, Reason: "term-overlap"}
	line := formatRelatedResult(res, featrelated.Options{Compact: false})

	assert.Contains(t, line, "(term-overlap)")
}
