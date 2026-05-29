package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineWriter_WriteLine_AppendsNewline(t *testing.T) {
	t.Parallel()

	// Arrange
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)

	// Act
	err := writer.WriteLine("hello")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello\n", buffer.String())
}

func TestLineWriter_WriteLine_SuppressesOutputInQuietMode(t *testing.T) {
	t.Parallel()

	// Arrange
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)
	writer.SetQuiet(true)

	// Act
	err := writer.WriteLine("should not appear")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, buffer.String())
}

func TestLineWriter_SetQuiet_ResumesOutputWhenDisabled(t *testing.T) {
	t.Parallel()

	// Arrange
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)
	writer.SetQuiet(true)
	_ = writer.WriteLine("suppressed")

	// Act
	writer.SetQuiet(false)
	_ = writer.WriteLine("visible")

	// Assert
	assert.Equal(t, "visible\n", buffer.String())
}

func TestLineWriter_WriteInline_WritesWithoutNewline(t *testing.T) {
	t.Parallel()

	// Arrange
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)

	// Act
	_ = writer.WriteInline("progress")

	// Assert
	assert.Equal(t, "progress", buffer.String())
}

func TestLineWriter_WriteInline_SuppressesOutputInQuietMode(t *testing.T) {
	t.Parallel()

	// Arrange
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)
	writer.SetQuiet(true)

	// Act
	_ = writer.WriteInline("hidden")

	// Assert
	assert.Empty(t, buffer.String())
}
