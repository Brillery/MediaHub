package server

import (
	"context"
	"testing"
	"time"

	"shorturl/pkg/log"
	"shorturl/pkg/utils"
	"shorturl/proto"
	"shorturl/shorturl-server/cache"
	mapdata "shorturl/shorturl-server/data"
)

func TestGetOriginalUrlReturnsNotFoundFromNegativeCache(t *testing.T) {
	key := utils.ToBase62(123)
	kvCache := newFakeKVCache(map[string]string{
		key: negativeCacheValue,
	})
	urlData := &fakeURLMapData{}
	service := newTestShortURLService(kvCache, urlData)

	_, err := service.GetOriginalUrl(context.Background(), &proto.ShortKey{Key: key, IsPublic: true})
	if err == nil {
		t.Fatal("GetOriginalUrl error = nil, want not found")
	}
	if urlData.getByIDCalls != 0 {
		t.Fatalf("GetByID calls = %d, want 0 when negative cache hits", urlData.getByIDCalls)
	}
	if urlData.incrementCalls != 0 {
		t.Fatalf("IncrementTimes calls = %d, want 0 when negative cache hits", urlData.incrementCalls)
	}
}

func TestIDFilterAllowsDBFallbackWhenMaxIDCacheMissing(t *testing.T) {
	kvCache := newFakeKVCache(nil)
	service := newTestShortURLService(kvCache, &fakeURLMapData{})

	if err := service.idFilter(123, kvCache, true); err != nil {
		t.Fatalf("idFilter error = %v, want nil when max_id cache is missing", err)
	}
}

func TestGetOriginalUrlWritesNegativeCacheOnDBMiss(t *testing.T) {
	key := utils.ToBase62(123)
	kvCache := newFakeKVCache(nil)
	urlData := &fakeURLMapData{}
	service := newTestShortURLService(kvCache, urlData)

	_, err := service.GetOriginalUrl(context.Background(), &proto.ShortKey{Key: key, IsPublic: true})
	if err == nil {
		t.Fatal("GetOriginalUrl error = nil, want not found")
	}
	if got := kvCache.values[key]; got != negativeCacheValue {
		t.Fatalf("negative cache value = %q, want %q", got, negativeCacheValue)
	}
	if got := kvCache.ttls[key]; got != negativeCacheTTLSeconds {
		t.Fatalf("negative cache ttl = %d, want %d", got, negativeCacheTTLSeconds)
	}
	if urlData.getByIDCalls != 1 {
		t.Fatalf("GetByID calls = %d, want 1", urlData.getByIDCalls)
	}
}

func newTestShortURLService(kvCache *fakeKVCache, urlData *fakeURLMapData) *shortUrlService {
	return &shortUrlService{
		log:               log.NewLogger(),
		urlMapDataFactory: &fakeURLMapDataFactory{urlData: urlData},
		kvCacheFactory:    &fakeCacheFactory{kvCache: kvCache},
		lockFactory:       &fakeLockFactory{lock: &fakeLock{locked: true}},
	}
}

type fakeKVCache struct {
	values map[string]string
	ttls   map[string]int
}

func newFakeKVCache(values map[string]string) *fakeKVCache {
	if values == nil {
		values = make(map[string]string)
	}
	return &fakeKVCache{
		values: values,
		ttls:   make(map[string]int),
	}
}

func (c *fakeKVCache) Get(key string) (string, error) {
	return c.values[key], nil
}

func (c *fakeKVCache) Set(key, value string, ttl int) error {
	c.values[key] = value
	c.ttls[key] = ttl
	return nil
}

func (c *fakeKVCache) Destroy() {}

type fakeCacheFactory struct {
	kvCache *fakeKVCache
}

func (f *fakeCacheFactory) NewKVCache() cache.KVCache {
	return f.kvCache
}

type fakeLock struct {
	locked bool
}

func (l *fakeLock) Lock(_ string, _ time.Duration) (bool, error) {
	return l.locked, nil
}

func (l *fakeLock) Unlock(_ string) error {
	return nil
}

type fakeLockFactory struct {
	lock *fakeLock
}

func (f *fakeLockFactory) NewDistributedLock() cache.DistributedLock {
	return f.lock
}

type fakeURLMapDataFactory struct {
	urlData *fakeURLMapData
}

func (f *fakeURLMapDataFactory) NewUrlMapData(_ bool) mapdata.IUrlMapData {
	return f.urlData
}

type fakeURLMapData struct {
	entity         *mapdata.UrlMapEntity
	getByIDCalls   int
	incrementCalls int
}

func (d *fakeURLMapData) GenerateID(_, _ int64) (int64, error) {
	return 0, nil
}

func (d *fakeURLMapData) Update(_ mapdata.UrlMapEntity) error {
	return nil
}

func (d *fakeURLMapData) GetByID(_ int64) (*mapdata.UrlMapEntity, error) {
	d.getByIDCalls++
	return d.entity, nil
}

func (d *fakeURLMapData) GetByOriginal(_ string) (mapdata.UrlMapEntity, error) {
	return mapdata.UrlMapEntity{}, nil
}

func (d *fakeURLMapData) IncrementTimes(_ int64, _ int, _ int64) error {
	d.incrementCalls++
	return nil
}

func (d *fakeURLMapData) GetTopUrls(_ int) ([]mapdata.UrlMapEntity, error) {
	return nil, nil
}
