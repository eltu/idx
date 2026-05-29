package filesystem_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

func TestCompositeIgnoreMatcher_Matches_ReturnsTrueWhenFirstMatcherMatches(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		alwaysMatchFactory{},
		neverMatchFactory{},
	)
	matcher, err := factory.New("/root")
	require.NoError(t, err)

	// Act
	matched, err := matcher.Matches("any/path.go")

	// Assert
	require.NoError(t, err)
	assert.True(t, matched)
}

func TestCompositeIgnoreMatcher_Matches_ReturnsTrueWhenSecondMatcherMatches(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		neverMatchFactory{},
		alwaysMatchFactory{},
	)
	matcher, err := factory.New("/root")
	require.NoError(t, err)

	// Act
	matched, err := matcher.Matches("any/path.go")

	// Assert
	require.NoError(t, err)
	assert.True(t, matched)
}

func TestCompositeIgnoreMatcher_Matches_ReturnsFalseWhenNoMatcherMatches(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		neverMatchFactory{},
		neverMatchFactory{},
	)
	matcher, err := factory.New("/root")
	require.NoError(t, err)

	// Act
	matched, err := matcher.Matches("any/path.go")

	// Assert
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestCompositeIgnoreMatcher_New_PropagatesFactoryError(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(errorFactory{})

	// Act
	_, err := factory.New("/root")

	// Assert
	require.Error(t, err)
}

func TestCompositeIgnoreMatcher_Matches_PropagatesMatcherError(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(errorOnMatchFactory{})
	matcher, err := factory.New("/root")
	require.NoError(t, err)

	// Act
	_, err = matcher.Matches("any/path.go")

	// Assert
	require.Error(t, err)
}

func TestCompositeIgnoreMatcher_WithGlobFactory_MatchesConfiguredPatterns(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		filesystem.NewGlobIgnoreMatcherFactory([]string{"*.tmp"}),
		filesystem.NewGlobIgnoreMatcherFactory([]string{"vendor/"}),
	)
	matcher, err := factory.New("/root")
	require.NoError(t, err)

	// Assert
	assertGlobMatches(t, matcher, "session.tmp", true)
	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

type alwaysMatchFactory struct{}

func (alwaysMatchFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	return alwaysMatcher{}, nil
}

type alwaysMatcher struct{}

func (alwaysMatcher) Matches(_ string) (bool, error) { return true, nil }

type neverMatchFactory struct{}

func (neverMatchFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	return neverMatcher{}, nil
}

type neverMatcher struct{}

func (neverMatcher) Matches(_ string) (bool, error) { return false, nil }

type errorFactory struct{}

func (errorFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	return nil, errors.New("factory init error")
}

type errorOnMatchFactory struct{}

func (errorOnMatchFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	return errorMatcher{}, nil
}

type errorMatcher struct{}

func (errorMatcher) Matches(_ string) (bool, error) {
	return false, errors.New("matcher error")
}
