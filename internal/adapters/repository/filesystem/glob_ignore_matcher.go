package filesystem

import (
	"fmt"
	"path/filepath"
	"strings"

	"idx/internal/core/ports"
)

// GlobIgnoreMatcherFactory builds a matcher from a set of glob patterns declared
// in index.ignore of .idx.yml.
//
// Example:
//
//	factory := NewGlobIgnoreMatcherFactory([]string{"*.tmp", "vendor/", "internal/gen/*.go"})
//	matcher, _ := factory.New(projectRoot)
//	matched, _ := matcher.Matches("cmd/gen/output.go") // false
type GlobIgnoreMatcherFactory struct {
	patterns []string
}

// NewGlobIgnoreMatcherFactory returns a factory for the given glob patterns.
func NewGlobIgnoreMatcherFactory(patterns []string) GlobIgnoreMatcherFactory {
	return GlobIgnoreMatcherFactory{patterns: patterns}
}

func (factory GlobIgnoreMatcherFactory) New(_ string) (ports.IgnoreMatcher, error) {
	compiled, err := compileGlobPatterns(factory.patterns)
	if err != nil {
		return nil, err
	}
	return globIgnoreMatcher{patterns: compiled}, nil
}

type compiledPattern struct {
	raw       string
	hasSlash  bool
}

type globIgnoreMatcher struct {
	patterns []compiledPattern
}

// Matches returns true when path matches any pattern in index.ignore.
// Patterns without a path separator are tested against the file's base name only,
// so "*.tmp" and "README.md" match in any directory.
// Patterns with a separator (e.g. "vendor/", "internal/gen/*.go") are tested
// against the full slash-normalised relative path.
func (matcher globIgnoreMatcher) Matches(path string) (bool, error) {
	normalized := filepath.ToSlash(path)
	base := filepath.Base(normalized)

	for _, p := range matcher.patterns {
		candidate := normalized
		if !p.hasSlash {
			candidate = base
		}

		matched, err := filepath.Match(p.raw, candidate)
		if err != nil {
			return false, fmt.Errorf("invalid ignore pattern %q: %w", p.raw, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

func compileGlobPatterns(patterns []string) ([]compiledPattern, error) {
	compiled := make([]compiledPattern, 0, len(patterns))
	for _, raw := range patterns {
		if raw == "" {
			continue
		}
		// Validate the pattern eagerly so misconfiguration is caught at startup.
		if _, err := filepath.Match(raw, ""); err != nil {
			return nil, fmt.Errorf("invalid ignore pattern %q: %w", raw, err)
		}
		normalized := filepath.ToSlash(strings.TrimSuffix(raw, "/"))
		compiled = append(compiled, compiledPattern{
			raw:      normalized,
			hasSlash: strings.Contains(normalized, "/"),
		})
	}
	return compiled, nil
}
