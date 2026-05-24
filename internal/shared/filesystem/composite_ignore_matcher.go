package filesystem

// CompositeIgnoreMatcherFactory combines multiple IgnoreMatcherBuilder instances.
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
	factories []IgnoreMatcherBuilder
}

// NewCompositeIgnoreMatcherFactory returns a factory that delegates to all provided factories.
func NewCompositeIgnoreMatcherFactory(factories ...IgnoreMatcherBuilder) CompositeIgnoreMatcherFactory {
	return CompositeIgnoreMatcherFactory{factories: factories}
}

func (factory CompositeIgnoreMatcherFactory) New(projectRoot string) (IgnoreMatcher, error) {
	matchers := make([]IgnoreMatcher, 0, len(factory.factories))
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
	matchers []IgnoreMatcher
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
