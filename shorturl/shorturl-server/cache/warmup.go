package cache

import (
	"context"
	"math/rand"
	"shorturl/pkg/log"
	"shorturl/shorturl-server/data"
	"strconv"
	"time"
)

// CacheWarmer 缓存预热接口
type CacheWarmer interface {
	// Warmup 预热缓存
	Warmup(ctx context.Context) error
	// StartPeriodicWarmup 启动定期预热
	StartPeriodicWarmup(ctx context.Context, interval time.Duration)
	// StopPeriodicWarmup 停止定期预热
	StopPeriodicWarmup()
}

// ShortUrlCacheWarmer 短链接缓存预热实现
type ShortUrlCacheWarmer struct {
	logger         log.ILogger
	kvCache        KVCache
	urlDataFactory data.IUrlMapDataFactory
	stopChan       chan struct{}
	bloomFilter    BloomFilter
}

// NewShortUrlCacheWarmer 创建一个新的短链接缓存预热器
func NewShortUrlCacheWarmer(logger log.ILogger, kvCache KVCache, urlDataFactory data.IUrlMapDataFactory, bloomFilter BloomFilter) CacheWarmer {
	return &ShortUrlCacheWarmer{
		logger:         logger,
		kvCache:        kvCache,
		urlDataFactory: urlDataFactory,
		stopChan:       make(chan struct{}),
		bloomFilter:    bloomFilter,
	}
}

// Warmup 预热缓存
func (w *ShortUrlCacheWarmer) Warmup(ctx context.Context) error {
	w.logger.Info("开始预热短链接缓存...")

	// 预热公共短链接
	if err := w.warmupPublicUrls(ctx); err != nil {
		w.logger.Error(err)
		return err
	}

	// 预热用户短链接
	if err := w.warmupUserUrls(ctx); err != nil {
		w.logger.Error(err)
		return err
	}

	w.logger.Info("短链接缓存预热完成")
	return nil
}

// warmupPublicUrls 预热公共短链接缓存
func (w *ShortUrlCacheWarmer) warmupPublicUrls(ctx context.Context) error {
	// 获取公共短链接数据访问对象
	d := w.urlDataFactory.NewUrlMapData(true)

	// 获取访问次数最多的前100个短链接
	urls, err := d.GetTopUrls(100)
	if err != nil {
		return err
	}

	// 将短链接数据预热到缓存
	for _, url := range urls {
		// 使用随机过期时间，避免缓存雪崩
		ttl := DefaultTTL*80/100 + rand.Intn(DefaultTTL*40/100)

		// 缓存原始URL
		key := url.ShortKey
		err := w.kvCache.Set(key, url.OriginalUrl, ttl)
		if err != nil {
			w.logger.Warning("预热缓存失败: " + err.Error())
			continue
		}

		// 将短链接ID添加到布隆过滤器（如果有布隆过滤器）
		if w.bloomFilter != nil {
			err := w.bloomFilter.Add("shorturl:bloom", strconv.FormatInt(url.ID, 10))
			if err != nil {
				w.logger.Warning("添加短链接到布隆过滤器失败: " + err.Error())
			}
		}
	}

	return nil
}

// warmupUserUrls 预热用户短链接缓存
func (w *ShortUrlCacheWarmer) warmupUserUrls(ctx context.Context) error {
	// 获取用户短链接数据访问对象
	d := w.urlDataFactory.NewUrlMapData(false)

	// 获取访问次数最多的前50个用户短链接
	urls, err := d.GetTopUrls(50)
	if err != nil {
		return err
	}

	// 将短链接数据预热到缓存
	for _, url := range urls {
		// 使用随机过期时间，避免缓存雪崩
		ttl := DefaultTTL*80/100 + rand.Intn(DefaultTTL*40/100)

		// 缓存原始URL
		key := "user_" + url.ShortKey
		err := w.kvCache.Set(key, url.OriginalUrl, ttl)
		if err != nil {
			w.logger.Warning("预热用户缓存失败: " + err.Error())
			continue
		}

		// 将短链接ID添加到布隆过滤器（如果有布隆过滤器）
		if w.bloomFilter != nil {
			err := w.bloomFilter.Add("shorturl:user:bloom", strconv.FormatInt(url.ID, 10))
			if err != nil {
				w.logger.Warning("添加用户短链接到布隆过滤器失败: " + err.Error())
			}
		}
	}

	return nil
}

// StartPeriodicWarmup 启动定期预热
func (w *ShortUrlCacheWarmer) StartPeriodicWarmup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := w.Warmup(ctx); err != nil {
					w.logger.Error(err)
				}
			case <-w.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// StopPeriodicWarmup 停止定期预热
func (w *ShortUrlCacheWarmer) StopPeriodicWarmup() {
	close(w.stopChan)
}
