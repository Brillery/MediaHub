package cron

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"testing"
	"time"
)

func TestFlushAccessCountsPreservesConcurrentIncrements(t *testing.T) {
	ctx := context.Background()
	tableName := "url_map"
	key := accessCountRedisKey(tableName)
	cache := newFakeAccessCountCache(true)
	cache.hashes[key] = map[string]int64{}
	cache.addBeforeDecrement[key] = map[string]int64{}
	cache.hashes[key]["123"] = 5
	cache.addBeforeDecrement[key]["123"] = 2
	data := &fakeAccessCountData{}
	now := int64(1710000000)

	if err := flushAccessCountsWithDeps(ctx, cache, data, []string{tableName}, now); err != nil {
		t.Fatalf("flushAccessCountsWithDeps error = %v, want nil", err)
	}

	if len(data.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(data.writes))
	}
	write := data.writes[0]
	if write.tableName != tableName || write.id != 123 || write.incrementTimes != 5 || write.now != now {
		t.Fatalf("write = %#v, want table/id/count/now snapshot", write)
	}
	if got := cache.hashes[key]["123"]; got != 2 {
		t.Fatalf("remaining redis count = %d, want concurrent remainder 2", got)
	}
	if cache.unlockCalls != 1 {
		t.Fatalf("unlock calls = %d, want 1", cache.unlockCalls)
	}
}

func TestFlushAccessCountsKeepsRedisCountWhenDatabaseFails(t *testing.T) {
	ctx := context.Background()
	tableName := "url_map"
	key := accessCountRedisKey(tableName)
	cache := newFakeAccessCountCache(true)
	cache.hashes[key] = map[string]int64{}
	cache.hashes[key]["456"] = 7
	data := &fakeAccessCountData{err: errors.New("mysql unavailable")}

	if err := flushAccessCountsWithDeps(ctx, cache, data, []string{tableName}, 1710000001); err == nil {
		t.Fatal("flushAccessCountsWithDeps error = nil, want database error")
	}

	if got := cache.hashes[key]["456"]; got != 7 {
		t.Fatalf("redis count after db error = %d, want unchanged 7", got)
	}
}

func TestFlushAccessCountsSkipsWhenLockNotAcquired(t *testing.T) {
	ctx := context.Background()
	tableName := "url_map"
	key := accessCountRedisKey(tableName)
	cache := newFakeAccessCountCache(false)
	cache.hashes[key] = map[string]int64{}
	cache.hashes[key]["789"] = 3
	data := &fakeAccessCountData{}

	if err := flushAccessCountsWithDeps(ctx, cache, data, []string{tableName}, 1710000002); err != nil {
		t.Fatalf("flushAccessCountsWithDeps error = %v, want nil when another instance owns lock", err)
	}

	if len(data.writes) != 0 {
		t.Fatalf("writes = %d, want 0 when lock is not acquired", len(data.writes))
	}
	if got := cache.hashes[key]["789"]; got != 3 {
		t.Fatalf("redis count = %d, want unchanged 3", got)
	}
}

type fakeAccessCountCache struct {
	locked             bool
	hashes             map[string]map[string]int64
	addBeforeDecrement map[string]map[string]int64
	unlockCalls        int
}

func newFakeAccessCountCache(locked bool) *fakeAccessCountCache {
	return &fakeAccessCountCache{
		locked:             locked,
		hashes:             map[string]map[string]int64{},
		addBeforeDecrement: map[string]map[string]int64{},
	}
}

func (c *fakeAccessCountCache) TryLock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return c.locked, nil
}

func (c *fakeAccessCountCache) Unlock(_ context.Context, _ string) error {
	c.unlockCalls++
	return nil
}

func (c *fakeAccessCountCache) Scan(_ context.Context, key string, _ uint64, _ int64) ([]string, uint64, error) {
	fields := c.hashes[key]
	names := make([]string, 0, len(fields))
	for field := range fields {
		names = append(names, field)
	}
	sort.Strings(names)

	entries := make([]string, 0, len(names)*2)
	for _, field := range names {
		entries = append(entries, field, strconv.FormatInt(fields[field], 10))
	}
	return entries, 0, nil
}

func (c *fakeAccessCountCache) Decrement(_ context.Context, key, field string, delta int64) (int64, error) {
	if c.hashes[key] == nil {
		c.hashes[key] = map[string]int64{}
	}
	if c.addBeforeDecrement[key] != nil {
		c.hashes[key][field] += c.addBeforeDecrement[key][field]
	}
	c.hashes[key][field] -= delta
	return c.hashes[key][field], nil
}

func (c *fakeAccessCountCache) DeleteIfNonPositive(_ context.Context, key, field string) error {
	if c.hashes[key][field] <= 0 {
		delete(c.hashes[key], field)
	}
	return nil
}

type fakeAccessCountData struct {
	err    error
	writes []accessCountWrite
}

func (d *fakeAccessCountData) IncrementTimes(tableName string, id int64, incrementTimes int64, now int64) error {
	if d.err != nil {
		return d.err
	}
	d.writes = append(d.writes, accessCountWrite{
		tableName:      tableName,
		id:             id,
		incrementTimes: incrementTimes,
		now:            now,
	})
	return nil
}

type accessCountWrite struct {
	tableName      string
	id             int64
	incrementTimes int64
	now            int64
}
