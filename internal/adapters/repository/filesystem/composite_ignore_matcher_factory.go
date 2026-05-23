package filesystem

import (
	"idx/internal/core/ports"
)

// CompositeIgnoreMatcherFactory combines multiple IgnoreMatcherFactory instances.
// The resulting matcher returns true (ignored) when any inner matcher matches.
//
// Example:
//
//	factory := NewCompositeIgnoreMatcherFactory(
//	    NewGitIgnoreMatcherFactory(),
//	    NewGlobIgnoreMatcherFactory(cfg.Index.Ignore),
//	)
//	matcher, _ := factory.New(projectRoot)
type CompositeIgnoreMatcherFactory struct {
	factories []ports.IgnoreMatcherFactory
}

// NewCompositeIgnoreMatcherFactory returns a factory that delegates to all provided factories.
func NewCompositeIgnoreMatcherFactory(factories ...ports.IgnoreMatcherFactory) CompositeIgnoreMatcherFactory {
	return CompositeIgnoreMatcherFactory{factories: factories}
}

func (factory CompositeIgnoreMatcherFactory) New(projectRoot string) (ports.IgnoreMatcher, error) {
	matchers := make([]ports.IgnoreMatcher, 0, len(factory.factories))
	for _, f := range factory.factories {
		m, err := f.New(projectRoot)
		if err != nil {
			return nil, err
		}
		matchers = append(matchers, m)
	}
	return compositeIgnoreMatcher{matchers: matchers}, nil
}

type compositeIgnoreMatcher struct {
	matchers []ports.IgnoreMatcher
}

func (matcher compositeIgnoreMatcher) Matches(path string) (bool, error) {
	for _, m := range matcher.matchers {
		matched, err := m.Matches(path)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
