// Package server 承载 shorturl gRPC 服务的核心业务编排。
//
// 本模块负责在短链生成、短链解析、Redis 缓存、布隆过滤器、分布式锁和 MySQL 数据访问之间做请求级协调。
// 输入来自 gRPC proto 请求；输出是 proto.Url 或明确错误。
// 状态边界：短链持久化状态归 data 层维护，缓存状态归 cache 层维护，本模块只决定何时读取、回填和降级。
// 并发边界：缓存击穿通过分布式锁收敛，同一个短链 key 在多实例并发 miss 时只允许一个请求优先回源；锁失败时允许直接回源兜底，保证可用性优先。
// 本模块不负责 HTTP 路由、不负责鉴权拦截器、不负责定时刷新 max_id，也不直接操作 Redis/MySQL SDK。
package server

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"shorturl/pkg/config"
	"shorturl/pkg/constants"
	"shorturl/pkg/log"
	"shorturl/pkg/utils"
	"shorturl/pkg/zerror"
	"shorturl/proto"
	"shorturl/shorturl-server/cache"
	"shorturl/shorturl-server/data"
	"strconv"
	"time"
)

const (
	// negativeCacheValue 是短链不存在时写入 Redis 的空值哨兵。
	//
	// 不能用空字符串表示“不存在”：KVCache.Get 返回空字符串也代表缓存未命中，两者混在一起会导致空值缓存永远不生效。
	negativeCacheValue = "__mediahub_shorturl_not_found__"

	// negativeCacheTTLSeconds 是空值缓存的短过期时间。
	//
	// 该值只用于抵挡不存在短码的重复访问，不应该像正常短链缓存一样保存 30 天，避免后续补数据后长时间不可见。
	negativeCacheTTLSeconds = 60
)

// shortUrlService 实现了 proto.ShortUrlServer 接口，提供短链接相关服务。
type shortUrlService struct {
	proto.UnimplementedShortUrlServer
	config               *config.Config // 配置信息
	log                  log.ILogger    // 日志记录器
	urlMapDataFactory    data.IUrlMapDataFactory
	kvCacheFactory       cache.CacheFactory
	lockFactory          cache.DistributedLockFactory
	bloomFactory         cache.BloomFilterFactory
	accessCounterFactory cache.AccessCounterFactory
	bloomFilter          cache.BloomFilter
	userBloomFilter      cache.BloomFilter
	cacheWarmer          cache.CacheWarmer
}

// NewService 创建一个新的短链接服务实例
func NewService(cnf *config.Config, logger log.ILogger, urlDataFactory data.IUrlMapDataFactory, kvCacheFactory cache.CacheFactory, lockFactory cache.DistributedLockFactory, bloomFactory cache.BloomFilterFactory, accessCounterFactory cache.AccessCounterFactory) proto.ShortUrlServer {
	// 创建缓存预热器
	kvCache := kvCacheFactory.NewKVCache()
	bloomFilter := bloomFactory.NewBloomFilter("shorturl:bloom", 100000, 0.01)
	userBloomFilter := bloomFactory.NewBloomFilter("shorturl:user:bloom", 100000, 0.01)

	// 创建缓存预热器
	cacheWarmer := cache.NewShortUrlCacheWarmer(logger, kvCache, urlDataFactory, bloomFilter)

	// 创建服务实例
	service := &shortUrlService{
		config:               cnf,
		log:                  logger,
		urlMapDataFactory:    urlDataFactory,
		kvCacheFactory:       kvCacheFactory,
		lockFactory:          lockFactory,
		bloomFactory:         bloomFactory,
		accessCounterFactory: accessCounterFactory,
		bloomFilter:          bloomFilter,
		userBloomFilter:      userBloomFilter,
		cacheWarmer:          cacheWarmer,
	}

	// 启动缓存预热
	service.startCacheWarmup()

	return service
}

// startCacheWarmup 启动缓存预热
func (s *shortUrlService) startCacheWarmup() {
	// 立即预热一次
	ctx := context.Background()
	if err := s.cacheWarmer.Warmup(ctx); err != nil {
		s.log.Error(err)
	}

	// 每6小时预热一次
	s.cacheWarmer.StartPeriodicWarmup(ctx, 6*time.Hour)
}

// GetShortUrl 根据原始URL生成或获取短链接
// 参数:
//
//	ctx: 上下文对象，用于控制请求的生命周期和取消机制
//	in: 包含原始URL和用户ID的请求对象，UserID为0时表示公共链接
//
// 返回:
//
//	*proto.Url: 包含生成的短链接地址和用户ID的响应对象
//	error: 生成或处理过程中发生的错误
func (s *shortUrlService) GetShortUrl(ctx context.Context, in *proto.Url) (*proto.Url, error) {
	isPublic := in.IsPublic
	if in.UserID != 0 {
		isPublic = false
	}

	// 参数有效性验证
	if in.Url == "" {
		err := zerror.NewByMsg("参数检查失败")
		s.log.Error(err)
		return nil, err
	}

	if !utils.IsUrl(in.Url) {
		err := zerror.NewByMsg("参数检查失败")
		s.log.Error(err)
		return nil, err
	}

	// 根据是否为公共链接创建数据访问对象
	d := s.urlMapDataFactory.NewUrlMapData(isPublic)
	entity, err := s.getOrCreateURLMapping(in, isPublic, d)
	if err != nil {
		s.log.Error(zerror.NewByErr(err))
		return nil, err
	}

	// 根据链接类型配置域名和缓存键前缀
	keyPrefix := ""
	domain := s.config.ShortDomain
	if !isPublic {
		keyPrefix = "user_"
		domain = s.config.UserShortDomain
	}

	// 缓存原始URL到分布式缓存
	kvCache := s.kvCacheFactory.NewKVCache()
	defer kvCache.Destroy()
	key := keyPrefix + entity.ShortKey

	// 使用随机过期时间，避免缓存雪崩
	ttl := randomShortURLCacheTTL()
	err = kvCache.Set(key, entity.OriginalUrl, ttl)
	if err != nil {
		s.log.Error(zerror.NewByErr(err))
		return nil, err
	}

	// 将短链接ID添加到布隆过滤器
	if s.bloomFilter != nil {
		if !isPublic {

			s.userBloomFilter.Add("", strconv.FormatInt(entity.ID, 10))
		} else {
			s.bloomFilter.Add("", strconv.FormatInt(entity.ID, 10))
		}
	}

	return &proto.Url{
		Url:    domain + entity.ShortKey,
		UserID: in.UserID,
	}, nil
}

// getOrCreateURLMapping 获取或创建原始 URL 对应的短链映射。
//
// 并发边界：首次查询未命中后，必须按原始 URL 获取分布式锁并在锁内二次查询。
// 这样多实例同时为同一个 URL 生成短链时，只有第一个请求会真正写入，其余请求会复用锁内已出现的记录。
func (s *shortUrlService) getOrCreateURLMapping(in *proto.Url, isPublic bool, d data.IUrlMapData) (data.UrlMapEntity, error) {
	entity, err := d.GetByOriginal(in.Url)
	if err != nil {
		return entity, err
	}
	if entity.ShortKey != "" {
		return entity, nil
	}
	return s.createURLMappingWithOriginalLock(in, isPublic, d)
}

// createURLMappingWithOriginalLock 在原始 URL 维度收敛短链创建。
//
// lock key 使用 SHA-256 摘要而不是原始 URL，避免把用户 URL 明文写入 Redis key。
// 如果锁服务异常，为了不让 Redis 锁故障阻断上传主链路，降级为直接创建；该降级仍可能产生重复记录，后续唯一索引迁移会进一步兜住。
func (s *shortUrlService) createURLMappingWithOriginalLock(in *proto.Url, isPublic bool, d data.IUrlMapData) (data.UrlMapEntity, error) {
	if s.lockFactory == nil {
		return s.createURLMappingDirect(in, d)
	}

	lockKey := originalURLCreationLockKey(isPublic, in.GetUserID(), in.Url)
	lock := s.lockFactory.NewDistributedLock()
	locked, err := lock.Lock(lockKey, 5*time.Second)
	if err != nil {
		s.log.Warning("获取短链创建锁失败，降级直接创建: " + err.Error())
		return s.createURLMappingDirect(in, d)
	}
	if locked {
		defer lock.Unlock(lockKey)

		// 获锁后必须二次查询：等待锁期间可能已有其他实例完成创建。
		entity, err := d.GetByOriginal(in.Url)
		if err != nil {
			return entity, err
		}
		if entity.ShortKey != "" {
			return entity, nil
		}
		return s.createURLMappingDirect(in, d)
	}

	// 未抢到锁时短暂等待持锁实例写入；仍未出现映射则直接创建，避免请求长期阻塞。
	time.Sleep(100 * time.Millisecond)
	entity, err := d.GetByOriginal(in.Url)
	if err != nil {
		return entity, err
	}
	if entity.ShortKey != "" {
		return entity, nil
	}
	return s.createURLMappingDirect(in, d)
}

// createURLMappingDirect 执行短链记录创建。
//
// 本函数只负责“生成 ID -> 计算 short key -> 回写记录”这条最小写入路径。
// 调用方负责在需要时先做原始 URL 维度的并发收敛。
func (s *shortUrlService) createURLMappingDirect(in *proto.Url, d data.IUrlMapData) (data.UrlMapEntity, error) {
	now := time.Now().Unix()
	id, err := d.GenerateID(in.GetUserID(), now)
	if err != nil {
		return data.UrlMapEntity{}, err
	}

	entity := data.UrlMapEntity{
		ID:          id,
		UserID:      in.GetUserID(),
		ShortKey:    utils.ToBase62(id),
		OriginalUrl: in.Url,
		UpdateAt:    now,
	}
	if err := d.Update(entity); err != nil {
		return data.UrlMapEntity{}, err
	}
	return entity, nil
}

// originalURLCreationLockKey 生成原始 URL 维度的短链创建锁 key。
//
// public 和 user 短链使用不同表，锁的冲突域也必须分开；私有短链还需要带 userID，避免不同用户的同一原始 URL 互相阻塞。
func originalURLCreationLockKey(isPublic bool, userID int64, originalURL string) string {
	scope := "public"
	if !isPublic {
		scope = fmt.Sprintf("user:%d", userID)
	}
	digest := sha256.Sum256([]byte(originalURL))
	return fmt.Sprintf("shorturl:create:%s:%x", scope, digest)
}

// GetOriginalUrl 根据短链接键获取原始URL
// 参数:
//
//	ctx context.Context: 上下文
//	in *proto.ShortKey: 包含短链接键和用户ID的请求参数
//
// 返回:
//
//	*proto.Url: 包含原始URL和用户ID的响应对象
//	error: 错误信息，若无错误则返回nil
func (s *shortUrlService) GetOriginalUrl(ctx context.Context, in *proto.ShortKey) (*proto.Url, error) {
	// 根据用户ID判断是否为公共链接
	isPublic := in.IsPublic
	if in.UserID != 0 {
		isPublic = false
	}

	// 参数有效性验证
	if in.Key == "" {
		err := zerror.NewByMsg("参数检查失败")
		s.log.Error(err)
		return nil, err
	}

	// 将短链接键转换为十进制ID
	id := utils.ToBase10(in.Key)
	if id == 0 {
		err := zerror.NewByMsg("参数检查失败")
		s.log.Error(err)
		return nil, err
	}

	// 根据是否为私有链接设置缓存键前缀
	keyPrefix := ""
	if !isPublic {
		keyPrefix = "user_"
	}

	// 创建并延迟销毁键值缓存实例
	kvCache := s.kvCacheFactory.NewKVCache()
	defer kvCache.Destroy()

	// 生成缓存键（格式：[user_] + 短链接键）
	key := keyPrefix + in.Key

	// 根据是否为公共链接创建对应的数据访问对象
	d := s.urlMapDataFactory.NewUrlMapData(isPublic)

	// 从缓存中获取原始URL
	originalUrl, err := kvCache.Get(key)
	if err != nil {
		s.log.Error(err)
		return nil, zerror.NewByErr(err)
	}
	if originalUrl == negativeCacheValue {
		err := zerror.NewByMsg("短链不存在")
		s.log.Error(err)
		return nil, err
	}

	// 如果缓存未命中，从数据库获取原始URL
	if originalUrl == "" {
		// 使用布隆过滤器检查短链接ID是否存在
		if s.bloomFilter != nil {
			if !isPublic {

				exists, err := s.userBloomFilter.Exists("", strconv.FormatInt(id, 10))
				if err != nil {
					s.log.Warning("布隆过滤器检查失败: " + err.Error())
				} else if !exists {
					// 布隆过滤器判断短链接不存在，直接返回错误
					err := zerror.NewByMsg("短链不存在")
					s.log.Error(err)
					return nil, err
				}
			} else {
				exists, err := s.bloomFilter.Exists("", strconv.FormatInt(id, 10))
				if err != nil {
					s.log.Warning("布隆过滤器检查失败: " + err.Error())
				} else if !exists {
					// 布隆过滤器判断短链接不存在，直接返回错误
					err := zerror.NewByMsg("短链不存在")
					s.log.Error(err)
					return nil, err
				}
			}
		}

		// 缓存穿透过滤
		err = s.idFilter(id, kvCache, isPublic)
		if err != nil {
			s.log.Error(err)
			return nil, err
		}

		originalUrl, err = s.resolveOriginalURLOnCacheMiss(id, key, kvCache, d)
		if err != nil {
			return nil, err
		}
	}

	// 访问计数是统计侧副作用，不能阻断短链跳转。
	// 这里只写 Redis 增量；MySQL times 由 shorturl-crontab 批量落库，避免高频访问同步打到数据库。
	s.recordAccessCount(isPublic, id)

	return &proto.Url{
		Url:    originalUrl,
		UserID: in.UserID,
	}, nil
}

// recordAccessCount 记录一次短链访问。
//
// 并发边界：多实例请求都通过 Redis HINCRBY 聚合到同一 table/id 字段；
// 这里不直接写 MySQL，crontab 会按快照扣减 Redis 并批量更新数据库，避免解析请求承担同步写库成本。
func (s *shortUrlService) recordAccessCount(isPublic bool, id int64) {
	if s.accessCounterFactory == nil {
		s.log.Warning("短链访问计数器未初始化，跳过本次计数")
		return
	}

	tableName := constants.TABLENAME_URL_MAP
	if !isPublic {
		tableName = constants.TABLENAME_URL_MAP_USER
	}

	counter := s.accessCounterFactory.NewAccessCounter()
	defer counter.Destroy()
	if err := counter.Increment(tableName, id); err != nil {
		s.log.Warning("记录短链访问计数失败: " + err.Error())
	}
}

// resolveOriginalURLOnCacheMiss 处理短链缓存未命中后的回源流程。
//
// 多实例并发访问同一短链时，优先用分布式锁收敛 DB 回源；拿不到锁的请求短暂等待后再读缓存。
// 如果锁服务异常，为了避免 Redis 锁故障放大成短链不可用，这里允许直接回源并回填缓存。
func (s *shortUrlService) resolveOriginalURLOnCacheMiss(id int64, key string, kvCache cache.KVCache, d data.IUrlMapData) (string, error) {
	lockKey := "lock:" + key
	lock := s.lockFactory.NewDistributedLock()
	locked, err := lock.Lock(lockKey, 5*time.Second)
	if err != nil {
		s.log.Warning("获取分布式锁失败，降级直接回源: " + err.Error())
		return s.loadOriginalURLFromDB(id, key, kvCache, d)
	}
	if locked {
		defer lock.Unlock(lockKey)

		// 获锁后必须再读一次缓存：等待锁期间可能已有其他实例完成回源并回填。
		originalURL, err := kvCache.Get(key)
		if err != nil {
			s.log.Error(err)
			return "", zerror.NewByErr(err)
		}
		if originalURL == negativeCacheValue {
			return "", zerror.NewByMsg("短链不存在")
		}
		if originalURL != "" {
			return originalURL, nil
		}
		return s.loadOriginalURLFromDB(id, key, kvCache, d)
	}

	// 未抢到锁时先给持锁实例一点回填时间；仍未命中再直接回源，避免请求长时间挂住。
	time.Sleep(100 * time.Millisecond)
	originalURL, err := kvCache.Get(key)
	if err != nil {
		s.log.Error(err)
		return "", zerror.NewByErr(err)
	}
	if originalURL == negativeCacheValue {
		return "", zerror.NewByMsg("短链不存在")
	}
	if originalURL != "" {
		return originalURL, nil
	}
	return s.loadOriginalURLFromDB(id, key, kvCache, d)
}

// loadOriginalURLFromDB 从 MySQL 回源并写入 Redis 缓存。
//
// 数据库未命中时写入短期空值哨兵，解决缓存穿透；数据库命中时写入带随机过期时间的正常缓存，降低雪崩概率。
func (s *shortUrlService) loadOriginalURLFromDB(id int64, key string, kvCache cache.KVCache, d data.IUrlMapData) (string, error) {
	entity, err := d.GetByID(id)
	if err != nil {
		s.log.Error(err)
		return "", zerror.NewByErr(err)
	}
	if entity == nil {
		if err := kvCache.Set(key, negativeCacheValue, negativeCacheTTLSeconds); err != nil {
			s.log.Warning("缓存空值失败: " + err.Error())
		}
		return "", zerror.NewByMsg("短链不存在")
	}

	originalURL := entity.OriginalUrl
	if err := kvCache.Set(key, originalURL, randomShortURLCacheTTL()); err != nil {
		s.log.Error(err)
		return "", zerror.NewByErr(err)
	}
	return originalURL, nil
}

// idFilter 验证短链ID是否合法
// 参数:
//
//	id: 需要验证的短链ID
//	kvCache: 用于存储和获取最大ID的键值缓存实例
//	isPublic: 是否为公共短链标识，true表示使用公共表，false使用用户表
//
// 返回值:
//
//	error: 验证失败时返回错误，nil表示验证通过
func (s *shortUrlService) idFilter(id int64, kvCache cache.KVCache, isPublic bool) error {
	key := fmt.Sprintf("%s_%s", constants.TABLENAME_URL_MAP, "max_id")
	// 根据是否为公共短链选择不同的最大ID缓存键
	if !isPublic {
		key = fmt.Sprintf("%s_%s", constants.TABLENAME_URL_MAP_USER, "max_id")
	}

	idStr, err := kvCache.Get(key)

	if err != nil {
		s.log.Error(err)
		return err
	}

	var rs int64
	// 从缓存中解析当前最大ID值
	if idStr != "" {
		rs, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			s.log.Error(err)
			return err
		}
	} else {
		// max_id 由独立 crontab 写入，服务刚启动、Redis 过期或定时任务延迟时可能暂时缺失。
		// 这里不能直接判定短链非法，必须放行到 DB 回源，否则会把合法短链误判成 404。
		return nil
	}

	// 验证传入ID是否小于等于当前最大合法ID
	if rs < id {
		err = zerror.NewByMsg("短链非法")
		s.log.Error(err)
		return err
	}
	return nil
}

// randomShortURLCacheTTL 返回带随机抖动的正常短链缓存 TTL。
//
// 多个短链如果同一秒批量写入缓存，固定过期时间会造成集中失效；抖动可以降低缓存雪崩概率。
func randomShortURLCacheTTL() int {
	return cache.DefaultTTL*80/100 + rand.Intn(cache.DefaultTTL*40/100)
}
