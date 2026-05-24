package search

import (
	"testing"
	"time"
)

func TestInitCacheStartsEmpty(t *testing.T) {
	cache := initCache()

	if cache.entries == nil {
		t.Fatal("expected initialized cache entries map, got nil")
	}
	if len(cache.entries) != 0 {
		t.Fatalf("expected empty cache, got %d entries", len(cache.entries))
	}
}

func TestCacheSetAndGetHit(t *testing.T) {
	cache := initCache()
	key := "k1"
	results := []searchResult{{directoryPath: "/repo", fileName: "a.go", score: 1.0}}

	cache.setInCache(key, results)

	got, ok := cache.getFromCache(key)
	if !ok {
		t.Fatal("expected cache hit, got miss")
	}
	if len(got) != 1 || got[0].fileName != "a.go" {
		t.Fatalf("unexpected cached results: %+v", got)
	}
}

func TestCacheGetMiss(t *testing.T) {
	cache := initCache()

	got, ok := cache.getFromCache("missing")
	if ok {
		t.Fatalf("expected cache miss, got hit with %+v", got)
	}
	if got != nil {
		t.Fatalf("expected nil result on miss, got %+v", got)
	}
}

func TestCacheGetExpiredDeletesEntry(t *testing.T) {
	cache := initCache()
	cache.entries["expired"] = cacheEntry{
		results:   []searchResult{{fileName: "old.go"}},
		expiresAt: time.Now().Add(-1 * time.Second),
	}

	_, ok := cache.getFromCache("expired")
	if ok {
		t.Fatal("expected expired cache miss")
	}
	if _, exists := cache.entries["expired"]; exists {
		t.Fatal("expected expired entry to be deleted")
	}
}

func TestCacheSetInCacheAssignsFutureTTL(t *testing.T) {
	cache := initCache()
	key := "ttl"

	before := time.Now()
	cache.setInCache(key, []searchResult{{fileName: "ttl.go"}})
	after := time.Now()

	entry, exists := cache.entries[key]
	if !exists {
		t.Fatal("expected cache entry to exist")
	}
	if !entry.expiresAt.After(after) {
		t.Fatalf("expected future expiration, got %v", entry.expiresAt)
	}

	minExpected := before.Add(time.Minute - 200*time.Millisecond)
	if entry.expiresAt.Before(minExpected) {
		t.Fatalf("expected expiration close to now+TTL, got %v", entry.expiresAt)
	}
}

func TestCacheRenewTTLExtendsExpiration(t *testing.T) {
	cache := initCache()
	key := "renew"
	cache.entries[key] = cacheEntry{
		results:   []searchResult{{fileName: "renew.go"}},
		expiresAt: time.Now().Add(100 * time.Millisecond),
	}

	oldExpiry := cache.entries[key].expiresAt
	time.Sleep(20 * time.Millisecond)
	cache.renewCacheTTL(key)

	newExpiry := cache.entries[key].expiresAt
	if !newExpiry.After(oldExpiry) {
		t.Fatalf("expected renewed expiry after old expiry, old=%v new=%v", oldExpiry, newExpiry)
	}
}

func TestCacheRenewTTLNoopForMissingKey(t *testing.T) {
	cache := initCache()

	cache.renewCacheTTL("missing")

	if len(cache.entries) != 0 {
		t.Fatalf("expected cache to remain empty, got %d entries", len(cache.entries))
	}
}

func TestGetCacheEntriesReturnsCopy(t *testing.T) {
	cache := initCache()
	cache.entries["k"] = cacheEntry{results: []searchResult{{fileName: "x.go"}}, expiresAt: time.Now().Add(time.Minute)}

	entries := cache.GetCacheEntries()
	entries["k"] = cacheEntry{results: []searchResult{{fileName: "changed.go"}}, expiresAt: time.Now().Add(time.Minute)}

	original := cache.entries["k"]
	if original.results[0].fileName != "x.go" {
		t.Fatalf("expected original cache unchanged, got %+v", original)
	}
}

func TestCacheKeyForIgnoresPaginationOnly(t *testing.T) {
	base := Options{
		Format:      OutputJSON,
		Context:     2,
		MatchesOnly: true,
		FilesOnly:   false,
		PathQueries: []string{"internal/core", "docs"},
		From:        0,
		Size:        10,
	}

	page2 := base
	page2.From = 20
	page2.Size = 5

	k1 := cacheKeyFor("module idx", base)
	k2 := cacheKeyFor("module idx", page2)

	if k1 != k2 {
		t.Fatalf("expected same key when only pagination changes, got %q vs %q", k1, k2)
	}
}

func TestCacheKeyForChangesWhenFunctionalOptionsChange(t *testing.T) {
	base := Options{Format: OutputText, Context: 0, PathQueries: []string{"internal"}}
	changed := base
	changed.Context = 1

	k1 := cacheKeyFor("module idx", base)
	k2 := cacheKeyFor("module idx", changed)

	if k1 == k2 {
		t.Fatalf("expected different keys when functional options change, got %q", k1)
	}
}
