package server

import (
	"context"
	"fmt"
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

// shortUrlService 实现了 proto.ShortUrlServer 接口，提供短链接相关服务。
type shortUrlService struct {
	proto.UnimplementedShortUrlServer
	config            *config.Config // 配置信息
	log               log.ILogger    // 日志记录器
	urlMapDataFactory data.IUrlMapDataFactory
	kvCacheFactory    cache.CacheFactory
}

// NewService 创建并返回一个新的 shortUrlService 实例。
func NewService(cnf *config.Config, log log.ILogger, urlMapDataFactory data.IUrlMapDataFactory, kvCacheFactory cache.CacheFactory) proto.ShortUrlServer {
	return &shortUrlService{
		config:            cnf,
		log:               log,
		urlMapDataFactory: urlMapDataFactory,
		kvCacheFactory:    kvCacheFactory,
	}
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
	entity, err := d.GetByOriginal(in.Url)
	if err != nil {
		s.log.Error(zerror.NewByErr(err))
		return nil, err
	}

	// 生成新的短链接标识符（若未存在）
	if entity.ShortKey == "" {
		id, err := d.GenerateID(in.GetUserID(), time.Now().Unix())
		if err != nil {
			s.log.Error(zerror.NewByErr(err))
			return nil, err
		}
		entity.ShortKey = utils.ToBase62(id)
		entity.OriginalUrl = in.Url
		entity.ID = id
		entity.UpdateAt = time.Now().Unix()
		err = d.Update(entity)
		if err != nil {
			s.log.Error(zerror.NewByErr(err))
			return nil, err
		}
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
	err = kvCache.Set(key, entity.OriginalUrl, cache.DefaultTTL)
	if err != nil {
		s.log.Error(zerror.NewByErr(err))
		return nil, err
	}
	return &proto.Url{
		Url:    domain + entity.ShortKey,
		UserID: in.UserID,
	}, nil
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

	// 如果缓存未命中，从数据库获取原始URL
	if originalUrl == "" {
		fmt.Println(id)
		// 缓存穿透过滤
		err = s.idFilter(id, kvCache, isPublic)
		if err != nil {
			s.log.Error(err)
			return nil, err
		}

		entity, err := d.GetByID(id)
		if err != nil {
			s.log.Error(err)
			return nil, zerror.NewByErr(err)
		}
		originalUrl = entity.OriginalUrl
	}

	// 将原始URL写入缓存
	err = kvCache.Set(key, originalUrl, cache.DefaultTTL)
	if err != nil {
		s.log.Error(err)
		return nil, zerror.NewByErr(err)
	}

	// 增加短链接访问次数（错误时仅记录日志不影响返回）
	err = d.IncrementTimes(id, 1, time.Now().Unix())
	if err != nil {
		s.log.Warning(err)
		err = nil
	}

	return &proto.Url{
		Url:    originalUrl,
		UserID: in.UserID,
	}, nil
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
	fmt.Printf("缓存键: %s, 缓存中的最大ID: %s, 当前请求ID: %d\n", key, idStr, id)

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
	}

	// 验证传入ID是否小于等于当前最大合法ID
	if rs < id {
		err = zerror.NewByMsg("短链非法")
		s.log.Error(err)
		return err
	}
	return nil
}
