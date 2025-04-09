package cache

// CacheFactory 是一个用于创建不同缓存实例的工厂接口。
// 通过实现该接口，可以定义不同类型的缓存创建逻辑。
type CacheFactory interface {
	// NewKVCache 创建并返回一个新的键值缓存实例。
	//
	// 返回：
	// KVCache：实现键值缓存操作的接口实例。
	NewKVCache() KVCache
}
