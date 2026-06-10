package config

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Key* constants are the canonical display names for all 13 configurable fields.
const (
	KeySearchFormat     = "search.format"
	KeySearchLimit      = "search.limit"
	KeySearchOperator   = "search.operator"
	KeySearchContext    = "search.context"
	KeySearchRelaxation = "search.relaxation"
	KeySearchCacheTTL   = "search.cache_ttl"
	KeySearchMaxWorkers = "search.max_workers"
	KeyWatchDebounce    = "watch.debounce"
	KeyIndexIgnore      = "index.ignore"
	KeyBM25K1           = "bm25.k1"
	KeyBM25B            = "bm25.b"
	KeyBM25ProxWeight   = "bm25.proximity_weight"
	KeyLogLevel         = "log.level"
)

// AllKeys returns the 13 configurable keys in display order.
// Example: for _, key := range config.AllKeys() { ... }.
func AllKeys() []string {
	return []string{
		KeySearchFormat, KeySearchLimit, KeySearchOperator, KeySearchContext,
		KeySearchRelaxation, KeySearchCacheTTL, KeySearchMaxWorkers,
		KeyWatchDebounce, KeyIndexIgnore,
		KeyBM25K1, KeyBM25B, KeyBM25ProxWeight,
		KeyLogLevel,
	}
}

// FieldValue returns cfg's value for key as a display string.
// Returns "" for unknown keys.
// Example: FieldValue(cfg, KeySearchFormat) → "json".
func FieldValue(cfg IdxConfig, key string) string {
	switch key {
	case KeySearchFormat:
		return cfg.Search.Format
	case KeySearchLimit:
		return fmt.Sprintf("%d", cfg.Search.Limit)
	case KeySearchOperator:
		return cfg.Search.Operator
	case KeySearchContext:
		return fmt.Sprintf("%d", cfg.Search.Context)
	case KeySearchRelaxation:
		return cfg.Search.Relaxation
	case KeySearchCacheTTL:
		return cfg.Search.CacheTTL.String()
	case KeySearchMaxWorkers:
		return fmt.Sprintf("%d", cfg.Search.MaxWorkers)
	case KeyWatchDebounce:
		return cfg.Watch.Debounce.String()
	case KeyIndexIgnore:
		return FormatIgnore(cfg.Index.Ignore)
	case KeyBM25K1:
		return FormatFloat(cfg.BM25.K1)
	case KeyBM25B:
		return FormatFloat(cfg.BM25.B)
	case KeyBM25ProxWeight:
		return FormatFloat(cfg.BM25.ProximityWeight)
	case KeyLogLevel:
		return cfg.Log.Level
	default:
		return ""
	}
}

// DefaultFieldValue returns the built-in default display string for key.
// Example: DefaultFieldValue(KeySearchFormat) → "text".
func DefaultFieldValue(key string) string {
	defaults := map[string]string{
		KeySearchFormat:     "text",
		KeySearchLimit:      "0",
		KeySearchOperator:   "AND",
		KeySearchContext:    "0",
		KeySearchRelaxation: "",
		KeySearchCacheTTL:   time.Minute.String(),
		KeySearchMaxWorkers: "4",
		KeyWatchDebounce:    (750 * time.Millisecond).String(),
		KeyIndexIgnore:      "[]",
		KeyBM25K1:           "1.5",
		KeyBM25B:            "0.75",
		KeyBM25ProxWeight:   "3",
		KeyLogLevel:         "error",
	}
	return defaults[key]
}

// FormatOutput writes a plain-text config table to w.
// Used by the server handler; the client renders it via output.Writer.
// Example: if err := config.FormatOutput(&sb, cfg, "/project/.idx.yml", overrides); err != nil { ... }.
func FormatOutput(w io.Writer, cfg IdxConfig, filePath string, overrides []string) error {
	if filePath == "" {
		return writef(w, "\n  No .idx.yml found — using built-in defaults.\n  Tip: create .idx.yml at the project root to customize defaults.\n\n")
	}

	overrideSet := make(map[string]bool, len(overrides))
	for _, k := range overrides {
		overrideSet[k] = true
	}

	if err := writef(w, "\n  Config  %s\n\n", filePath); err != nil {
		return err
	}

	keys := AllKeys()
	maxKey, maxVal := configColumnWidths(cfg, keys)
	for _, key := range keys {
		val := FieldValue(cfg, key)
		def := DefaultFieldValue(key)
		var err error
		if overrideSet[key] {
			err = writef(w, "  %s  %s  ← .idx.yml   (default: %s)\n", PadRight(key, maxKey), PadRight(val, maxVal), def)
		} else {
			err = writef(w, "  %s  %s  · default\n", PadRight(key, maxKey), PadRight(val, maxVal))
		}
		if err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writef(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

func configColumnWidths(cfg IdxConfig, keys []string) (int, int) {
	maxKey, maxVal := 0, 0
	for _, key := range keys {
		if len(key) > maxKey {
			maxKey = len(key)
		}
		if val := FieldValue(cfg, key); len(val) > maxVal {
			maxVal = len(val)
		}
	}
	return maxKey, maxVal
}

// FormatFloat formats a float64 for config display.
// Example: FormatFloat(1.5) → "1.5".
func FormatFloat(f float64) string {
	return fmt.Sprintf("%.4g", f)
}

// FormatIgnore formats a list of ignore patterns for config display.
// Example: FormatIgnore([]string{"vendor/"}) → "[vendor/]".
func FormatIgnore(patterns []string) string {
	if len(patterns) == 0 {
		return "[]"
	}
	return "[" + strings.Join(patterns, ", ") + "]"
}

// PadRight pads s with spaces on the right to reach width.
// Example: PadRight("key", 10) → "key       ".
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
