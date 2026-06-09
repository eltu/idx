package search

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCache_StartsEmpty(t *testing.T) {
	t.Parallel()

	// Act
	cache := initCache()

	// Assert
	require.NotNil(t, cache.entries)
	assert.Empty(t, cache.entries)
}

func TestCache_SetAndGet_ReturnsStoredResult(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()
	key := "k1"
	results := []searchResult{{directoryPath: "/repo", fileName: "a.go", score: 1.0}}

	// Act
	cache.setInCache(key, results)
	got, ok := cache.getFromCache(key)

	// Assert
	require.True(t, ok, "expected cache hit")
	require.Len(t, got, 1)
	assert.Equal(t, "a.go", got[0].fileName)
}

func TestCache_Get_ReturnsMissForMissingKey(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()

	// Act
	got, ok := cache.getFromCache("missing")

	// Assert
	assert.False(t, ok)
	assert.Nil(t, got)
}

func TestCache_Get_DeletesExpiredEntry(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()
	cache.entries["expired"] = cacheEntry{
		results:   []searchResult{{fileName: "old.go"}},
		expiresAt: time.Now().Add(-1 * time.Second),
	}

	// Act
	_, ok := cache.getFromCache("expired")

	// Assert
	assert.False(t, ok, "expected expired cache miss")
	_, exists := cache.entries["expired"]
	assert.False(t, exists, "expected expired entry to be deleted")
}

func TestCache_SetInCache_AssignsFutureTTL(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()
	key := "ttl"
	before := time.Now()

	// Act
	cache.setInCache(key, []searchResult{{fileName: "ttl.go"}})
	after := time.Now()

	// Assert
	entry, exists := cache.entries[key]
	require.True(t, exists)
	assert.True(t, entry.expiresAt.After(after), "expected future expiration")
	assert.True(t, entry.expiresAt.After(before.Add(time.Minute-200*time.Millisecond)),
		"expected expiration close to now+TTL, got %v", entry.expiresAt)
}

func TestCache_RenewTTL_ExtendsExpiration(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()
	key := "renew"
	cache.entries[key] = cacheEntry{
		results:   []searchResult{{fileName: "renew.go"}},
		expiresAt: time.Now().Add(100 * time.Millisecond),
	}
	oldExpiry := cache.entries[key].expiresAt

	// Act
	cache.renewCacheTTL(key)

	// Assert
	newExpiry := cache.entries[key].expiresAt
	assert.True(t, newExpiry.After(oldExpiry), "expected renewed expiry after old expiry, old=%v new=%v", oldExpiry, newExpiry)
}

func TestCache_RenewTTL_NoopForMissingKey(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()

	// Act
	cache.renewCacheTTL("missing")

	// Assert
	assert.Empty(t, cache.entries)
}

func TestCache_GetCacheEntries_ReturnsCopy(t *testing.T) {
	t.Parallel()

	// Arrange
	cache := initCache()
	cache.entries["k"] = cacheEntry{
		results:   []searchResult{{fileName: "x.go"}},
		expiresAt: time.Now().Add(time.Minute),
	}

	// Act
	entries := cache.GetCacheEntries()
	entries["k"] = cacheEntry{results: []searchResult{{fileName: "changed.go"}}, expiresAt: time.Now().Add(time.Minute)}

	// Assert — original cache must be unaffected
	assert.Equal(t, "x.go", cache.entries["k"].results[0].fileName)
}

func TestCacheKeyFor_IgnoresPaginationOnly(t *testing.T) {
	t.Parallel()

	// Arrange
	base := Options{
		Format:      OutputJSON,
		Context:     2,
		FilesOnly:   false,
		PathQueries: []string{"internal/core", "docs"},
		From:        0,
		Size:        10,
	}
	page2 := base
	page2.From = 20
	page2.Size = 5

	// Act
	k1 := cacheKeyFor("module idx", base)
	k2 := cacheKeyFor("module idx", page2)

	// Assert
	assert.Equal(t, k1, k2, "expected same key when only pagination changes")
}

func TestCacheKeyFor_ChangesWhenFunctionalOptionsChange(t *testing.T) {
	t.Parallel()

	// Arrange
	base := Options{Format: OutputText, Context: 0, PathQueries: []string{"internal"}}
	changed := base
	changed.Context = 1

	// Act
	k1 := cacheKeyFor("module idx", base)
	k2 := cacheKeyFor("module idx", changed)

	// Assert
	assert.NotEqual(t, k1, k2)
}
