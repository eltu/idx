package filesystem_test

import (
	"testing"

	"idx/internal/adapters/repository/filesystem"
)

func TestGlobIgnoreMatcherExtensionPatternMatchesAnyDirectory(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"*.tmp"})

	assertGlobMatches(t, matcher, "work/session.tmp", true)
	assertGlobMatches(t, matcher, "deep/nested/dir/output.tmp", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

func TestGlobIgnoreMatcherFilenamePatternMatchesAnyDirectory(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"config.go"})

	assertGlobMatches(t, matcher, "config.go", true)
	assertGlobMatches(t, matcher, "internal/core/domain/config.go", true)
	assertGlobMatches(t, matcher, "service.go", false)
}

func TestGlobIgnoreMatcherPathPatternMatchesExactRelativePath(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"internal/core/domain/config.go"})

	assertGlobMatches(t, matcher, "internal/core/domain/config.go", true)
	assertGlobMatches(t, matcher, "config.go", false)
	assertGlobMatches(t, matcher, "other/core/domain/config.go", false)
}

func TestGlobIgnoreMatcherPathPatternWithGlobMatchesDirectory(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"internal/gen/*.go"})

	assertGlobMatches(t, matcher, "internal/gen/schema.go", true)
	assertGlobMatches(t, matcher, "internal/core/schema.go", false)
}

func TestGlobIgnoreMatcherDirectoryPatternMatchesTrailingSlash(t *testing.T) {
	// vendor/ in index.ignore should ignore the vendor directory entry
	matcher := newGlobMatcher(t, []string{"vendor/"})

	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "vendor/lib.go", false) // directory itself, not its children
}

func TestGlobIgnoreMatcherEmptyPatternsNeverMatches(t *testing.T) {
	matcher := newGlobMatcher(t, []string{})

	assertGlobMatches(t, matcher, "anything.go", false)
}

func TestGlobIgnoreMatcherSkipsEmptyStringPatterns(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"", "*.log"})

	assertGlobMatches(t, matcher, "app.log", true)
	assertGlobMatches(t, matcher, "app.go", false)
}

func TestGlobIgnoreMatcherMultiplePatternsAnyMatchIgnores(t *testing.T) {
	matcher := newGlobMatcher(t, []string{"*.tmp", "vendor/", "debug.log"})

	assertGlobMatches(t, matcher, "session.tmp", true)
	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "debug.log", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

func TestGlobIgnoreMatcherFactoryInvalidPatternReturnsError(t *testing.T) {
	factory := filesystem.NewGlobIgnoreMatcherFactory([]string{"[invalid"})
	_, err := factory.New("/any/root")
	if err == nil {
		t.Fatal("expected error for invalid glob pattern, got nil")
	}
}

// newGlobMatcher is a test helper that builds a GlobIgnoreMatcher or fatals on error.
func newGlobMatcher(t *testing.T, patterns []string) interface {
	Matches(string) (bool, error)
} {
	t.Helper()
	factory := filesystem.NewGlobIgnoreMatcherFactory(patterns)
	matcher, err := factory.New("/irrelevant")
	if err != nil {
		t.Fatalf("NewGlobIgnoreMatcherFactory.New: unexpected error: %v", err)
	}
	return matcher
}

func assertGlobMatches(t *testing.T, matcher interface{ Matches(string) (bool, error) }, path string, want bool) {
	t.Helper()
	got, err := matcher.Matches(path)
	if err != nil {
		t.Fatalf("Matches(%q): unexpected error: %v", path, err)
	}
	if got != want {
		t.Errorf("Matches(%q) = %v, want %v", path, got, want)
	}
}
