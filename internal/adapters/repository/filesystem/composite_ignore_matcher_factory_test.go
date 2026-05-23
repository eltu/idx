package filesystem_test

import (
	"errors"
	"testing"

	"idx/internal/adapters/repository/filesystem"
	"idx/internal/core/ports"
)

func TestCompositeIgnoreMatcherReturnsTrueWhenFirstMatcherMatches(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		alwaysMatchFactory{},
		neverMatchFactory{},
	)
	matcher, err := factory.New("/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matched, err := matcher.Matches("any/path.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected match from first factory, got false")
	}
}

func TestCompositeIgnoreMatcherReturnsTrueWhenSecondMatcherMatches(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		neverMatchFactory{},
		alwaysMatchFactory{},
	)
	matcher, err := factory.New("/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matched, err := matcher.Matches("any/path.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !matched {
		t.Error("expected match from second factory, got false")
	}
}

func TestCompositeIgnoreMatcherReturnsFalseWhenNoMatcherMatches(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		neverMatchFactory{},
		neverMatchFactory{},
	)
	matcher, err := factory.New("/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	matched, err := matcher.Matches("any/path.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Error("expected no match, got true")
	}
}

func TestCompositeIgnoreMatcherPropagatesFactoryError(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		errorFactory{},
	)
	_, err := factory.New("/root")
	if err == nil {
		t.Fatal("expected error from inner factory, got nil")
	}
}

func TestCompositeIgnoreMatcherPropagatesMatcherError(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		errorOnMatchFactory{},
	)
	matcher, err := factory.New("/root")
	if err != nil {
		t.Fatalf("unexpected factory error: %v", err)
	}
	_, err = matcher.Matches("any/path.go")
	if err == nil {
		t.Fatal("expected error from inner matcher, got nil")
	}
}

func TestCompositeIgnoreMatcherWithGlobFactoryIntegration(t *testing.T) {
	factory := filesystem.NewCompositeIgnoreMatcherFactory(
		filesystem.NewGlobIgnoreMatcherFactory([]string{"*.tmp"}),
		filesystem.NewGlobIgnoreMatcherFactory([]string{"vendor/"}),
	)
	matcher, err := factory.New("/root")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertGlobMatches(t, matcher, "session.tmp", true)
	assertGlobMatches(t, matcher, "vendor", true)
	assertGlobMatches(t, matcher, "main.go", false)
}

// --- test doubles ---

type alwaysMatchFactory struct{}

func (alwaysMatchFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return alwaysMatcher{}, nil
}

type alwaysMatcher struct{}

func (alwaysMatcher) Matches(_ string) (bool, error) { return true, nil }

type neverMatchFactory struct{}

func (neverMatchFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return neverMatcher{}, nil
}

type neverMatcher struct{}

func (neverMatcher) Matches(_ string) (bool, error) { return false, nil }

type errorFactory struct{}

func (errorFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return nil, errors.New("factory init error")
}

type errorOnMatchFactory struct{}

func (errorOnMatchFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return errorMatcher{}, nil
}

type errorMatcher struct{}

func (errorMatcher) Matches(_ string) (bool, error) {
	return false, errors.New("matcher error")
}
