package cache

import (
	"testing"
	"time"
)

// stubDistributedCache 模拟即将过期的 Redis 值；接口没有剩余 TTL，调用方不能续期。
type stubDistributedCache struct{ value string }

func (c *stubDistributedCache) Get(string) (string, error)          { return c.value, nil }
func (c *stubDistributedCache) Set(_ string, v string, _ int) error { c.value = v; return nil }
func (c *stubDistributedCache) Destroy()                            {}
func TestDistributedReadDoesNotExtendUnknownTTL(t *testing.T) {
	local := &MemoryCache{cache: make(map[string]cacheItem)}
	remote := &stubDistributedCache{value: "missing"}
	c := NewTwoLevelCache(local, remote, nil)
	if _, err := c.Get("key"); err != nil {
		t.Fatal(err)
	}
	remote.value = ""
	if got, _ := c.Get("key"); got != "" {
		t.Fatalf("expired Redis value retained locally: %q", got)
	}
}
func TestTwoLevelCacheSupportsOneSecondTTL(t *testing.T) {
	local := &MemoryCache{cache: make(map[string]cacheItem)}
	c := NewTwoLevelCache(local, &stubDistributedCache{}, nil)
	before := time.Now()
	if err := c.Set("key", "value", 1); err != nil {
		t.Fatal(err)
	}
	if expiry := local.cache["key"].expiration; expiry.After(before.Add(1100 * time.Millisecond)) {
		t.Fatalf("local expiry exceeds requested TTL: %v", expiry)
	}
}
