package server

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"shorturl/pkg/config"
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

func TestGetShortUrlRechecksOriginalURLAfterCreationLock(t *testing.T) {
	originalURL := "https://img.example.com/reused.jpg"
	kvCache := newFakeKVCache(nil)
	urlData := &fakeURLMapData{
		originalResults: []mapdata.UrlMapEntity{
			{},
			{ID: 88, ShortKey: "1q", OriginalUrl: originalURL},
		},
	}
	lock := &fakeLock{locked: true}
	service := newTestShortURLServiceWithLock(kvCache, urlData, lock)

	out, err := service.GetShortUrl(context.Background(), &proto.Url{Url: originalURL, IsPublic: true})
	if err != nil {
		t.Fatalf("GetShortUrl error = %v, want nil", err)
	}
	if out.GetUrl() != "https://short.example/1q" {
		t.Fatalf("short url = %q, want %q", out.GetUrl(), "https://short.example/1q")
	}
	if urlData.getByOriginalCalls != 2 {
		t.Fatalf("GetByOriginal calls = %d, want 2", urlData.getByOriginalCalls)
	}
	if urlData.generateCalls != 0 || urlData.updateCalls != 0 {
		t.Fatalf("GenerateID/Update calls = %d/%d, want 0/0", urlData.generateCalls, urlData.updateCalls)
	}
	if got := kvCache.values["1q"]; got != originalURL {
		t.Fatalf("cache value = %q, want %q", got, originalURL)
	}
	if len(lock.keys) != 1 {
		t.Fatalf("lock keys = %#v, want exactly one creation lock", lock.keys)
	}
	if strings.Contains(lock.keys[0], originalURL) {
		t.Fatalf("lock key %q should not contain raw original URL", lock.keys[0])
	}
}

func TestGetShortUrlFallsBackToDirectCreateWhenCreationLockErrors(t *testing.T) {
	originalURL := "https://img.example.com/new.jpg"
	kvCache := newFakeKVCache(nil)
	urlData := &fakeURLMapData{generatedID: 321}
	lock := &fakeLock{lockErr: errors.New("redis unavailable")}
	service := newTestShortURLServiceWithLock(kvCache, urlData, lock)

	out, err := service.GetShortUrl(context.Background(), &proto.Url{Url: originalURL, IsPublic: true})
	if err != nil {
		t.Fatalf("GetShortUrl error = %v, want nil", err)
	}
	wantShortKey := utils.ToBase62(321)
	if out.GetUrl() != "https://short.example/"+wantShortKey {
		t.Fatalf("short url = %q, want %q", out.GetUrl(), "https://short.example/"+wantShortKey)
	}
	if urlData.generateCalls != 1 || urlData.updateCalls != 1 {
		t.Fatalf("GenerateID/Update calls = %d/%d, want 1/1", urlData.generateCalls, urlData.updateCalls)
	}
	if urlData.updatedEntity.OriginalUrl != originalURL {
		t.Fatalf("updated original url = %q, want %q", urlData.updatedEntity.OriginalUrl, originalURL)
	}
}

func newTestShortURLService(kvCache *fakeKVCache, urlData *fakeURLMapData) *shortUrlService {
	return newTestShortURLServiceWithLock(kvCache, urlData, &fakeLock{locked: true})
}

func newTestShortURLServiceWithLock(kvCache *fakeKVCache, urlData *fakeURLMapData, lock *fakeLock) *shortUrlService {
	return &shortUrlService{
		config: &config.Config{
			ShortDomain:     "https://short.example/",
			UserShortDomain: "https://user-short.example/",
		},
		log:               log.NewLogger(),
		urlMapDataFactory: &fakeURLMapDataFactory{urlData: urlData},
		kvCacheFactory:    &fakeCacheFactory{kvCache: kvCache},
		lockFactory:       &fakeLockFactory{lock: lock},
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
	locked  bool
	lockErr error
	keys    []string
}

func (l *fakeLock) Lock(key string, _ time.Duration) (bool, error) {
	l.keys = append(l.keys, key)
	if l.lockErr != nil {
		return false, l.lockErr
	}
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
	entity             *mapdata.UrlMapEntity
	originalResults    []mapdata.UrlMapEntity
	generatedID        int64
	updatedEntity      mapdata.UrlMapEntity
	getByIDCalls       int
	getByOriginalCalls int
	generateCalls      int
	updateCalls        int
	incrementCalls     int
}

func (d *fakeURLMapData) GenerateID(_, _ int64) (int64, error) {
	d.generateCalls++
	if d.generatedID != 0 {
		return d.generatedID, nil
	}
	return 1, nil
}

func (d *fakeURLMapData) Update(entity mapdata.UrlMapEntity) error {
	d.updateCalls++
	d.updatedEntity = entity
	return nil
}

func (d *fakeURLMapData) GetByID(_ int64) (*mapdata.UrlMapEntity, error) {
	d.getByIDCalls++
	return d.entity, nil
}

func (d *fakeURLMapData) GetByOriginal(_ string) (mapdata.UrlMapEntity, error) {
	d.getByOriginalCalls++
	if len(d.originalResults) > 0 {
		entity := d.originalResults[0]
		d.originalResults = d.originalResults[1:]
		return entity, nil
	}
	return mapdata.UrlMapEntity{}, nil
}

func (d *fakeURLMapData) IncrementTimes(_ int64, _ int, _ int64) error {
	d.incrementCalls++
	return nil
}

func (d *fakeURLMapData) GetTopUrls(_ int) ([]mapdata.UrlMapEntity, error) {
	return nil, nil
}
