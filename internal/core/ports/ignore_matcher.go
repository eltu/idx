package ports

type IgnoreMatcher interface {
	Matches(path string) (bool, error)
}

type IgnoreMatcherFactory interface {
	New(projectRoot string) (IgnoreMatcher, error)
}
