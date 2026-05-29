package filesystem_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

func TestGlobIgnoreMatcher_Matches_ExtensionPatternMatchesAnyDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"*.tmp"})

	// Assert
	assertGlobMatches(t, matcher, "work/session.tmp", true)
	assertGlobMatches(t, matcher, "deep/nested/dir/output.tmp", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

func TestGlobIgnoreMatcher_Matches_FilenamePatternMatchesAnyDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"config.go"})

	// Assert
	assertGlobMatches(t, matcher, "config.go", true)
	assertGlobMatches(t, matcher, "internal/core/domain/config.go", true)
	assertGlobMatches(t, matcher, "service.go", false)
}

func TestGlobIgnoreMatcher_Matches_PathPatternMatchesExactRelativePath(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"internal/core/domain/config.go"})

	// Assert
	assertGlobMatches(t, matcher, "internal/core/domain/config.go", true)
	assertGlobMatches(t, matcher, "config.go", false)
	assertGlobMatches(t, matcher, "other/core/domain/config.go", false)
}

func TestGlobIgnoreMatcher_Matches_PathPatternWithGlobMatchesDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"internal/gen/*.go"})

	// Assert
	assertGlobMatches(t, matcher, "internal/gen/schema.go", true)
	assertGlobMatches(t, matcher, "internal/core/schema.go", false)
}

func TestGlobIgnoreMatcher_Matches_DirectoryPatternMatchesTrailingSlash(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"vendor/"})

	// Assert
	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "vendor/lib.go", false)
}

func TestGlobIgnoreMatcher_Matches_EmptyPatternsNeverMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{})

	// Assert
	assertGlobMatches(t, matcher, "anything.go", false)
}

func TestGlobIgnoreMatcher_Matches_SkipsEmptyStringPatterns(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"", "*.log"})

	// Assert
	assertGlobMatches(t, matcher, "app.log", true)
	assertGlobMatches(t, matcher, "app.go", false)
}

func TestGlobIgnoreMatcher_Matches_MultiplePatternsAnyMatchIgnores(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := newGlobMatcher(t, []string{"*.tmp", "vendor/", "debug.log"})

	// Assert
	assertGlobMatches(t, matcher, "session.tmp", true)
	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "debug.log", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

func TestGlobIgnoreMatcherFactory_New_ReturnsErrorForInvalidPattern(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewGlobIgnoreMatcherFactory([]string{"[invalid"})

	// Act
	_, err := factory.New("/any/root")

	// Assert
	require.Error(t, err)
}

func newGlobMatcher(t *testing.T, patterns []string) interface {
	Matches(string) (bool, error)
} {
	t.Helper()
	factory := filesystem.NewGlobIgnoreMatcherFactory(patterns)
	matcher, err := factory.New("/irrelevant")
	require.NoError(t, err)
	return matcher
}

func assertGlobMatches(t *testing.T, matcher interface{ Matches(string) (bool, error) }, path string, want bool) {
	t.Helper()
	got, err := matcher.Matches(path)
	require.NoError(t, err, "Matches(%q)", path)
	assert.Equal(t, want, got, "Matches(%q)", path)
}
