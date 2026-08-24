package utils

import (
	"testing"
	"time"
)

func TestAssistantSuggestionCacheCopiesExpiresAndInvalidates(t *testing.T) {
	cache := &AssistantSuggestionCache{
		cache: make(map[string]assistantSuggestionCacheEntry),
	}
	original := []string{"How much was {item}?"}

	cache.Set("user-1", original, time.Hour)
	original[0] = "changed"

	cached, ok := cache.Get("user-1")
	if !ok || cached[0] != "How much was {item}?" {
		t.Fatalf("unexpected cached suggestions: %#v, ok=%v", cached, ok)
	}

	cached[0] = "mutated"
	again, _ := cache.Get("user-1")
	if again[0] != "How much was {item}?" {
		t.Fatalf("cache returned shared mutable data: %#v", again)
	}

	cache.Invalidate("user-1")
	if _, ok := cache.Get("user-1"); ok {
		t.Fatal("cache entry still exists after invalidation")
	}

	cache.Set("user-2", []string{"Where was {item}?"}, -time.Second)
	if _, ok := cache.Get("user-2"); ok {
		t.Fatal("expired cache entry was returned")
	}
}
