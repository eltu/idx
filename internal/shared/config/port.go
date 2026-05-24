package config

// Repository loads .idx.yml from a project root directory.
// Example: cfg, overrides, err := repo.Load("/path/to/project").
type Repository interface {
	// Load reads .idx.yml from projectRoot, returning the resolved config and
	// the flat list of YAML keys explicitly set (e.g. "search.format", "bm25.k1").
	// If the file is absent, returns DefaultIdxConfig() with an empty overrides slice.
	Load(projectRoot string) (IdxConfig, []string, error)

	// FilePath returns the absolute path to .idx.yml if the file exists at
	// projectRoot, or an empty string when no config file is present.
	FilePath(projectRoot string) string
}
