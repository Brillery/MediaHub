package redis

import (
	"context"
	"time"
)

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	// RecordCacheAccess 记录缓存访问
	RecordCacheAccess(key string, hit bool, duration time.Duration)
}

// Logger 日志接口
type Logger interface {
	// Info 记录信息日志
	Info(msg string, fields ...interface{})
	// Error 记录错误日志
	Error(msg string, fields ...interface{})
}

// DefaultMetricsCollector 默认指标收集器
type DefaultMetricsCollector struct{}

// RecordCacheAccess 记录缓存访问
func (m *DefaultMetricsCollector) RecordCacheAccess(key string, hit bool, duration time.Duration) {
	// 默认实现为空，实际项目中可以实现为Prometheus指标等
}

// DefaultLogger 默认日志器
type DefaultLogger struct{}

// Info 记录信息日志
func (l *DefaultLogger) Info(msg string, fields ...interface{}) {
	// 默认实现为空，实际项目中可以实现为结构化日志等
}

// Error 记录错误日志
func (l *DefaultLogger) Error(msg string, fields ...interface{}) {
	// 默认实现为空，实际项目中可以实现为结构化日志等
}

// Decorator 缓存装饰器
type Decorator struct {
	strategy  CacheStrategy
	metrics   MetricsCollector
	logger    Logger
	defaultTTL time.Duration
}

// NewDecorator 创建缓存装饰器
func NewDecorator(strategy CacheStrategy, metrics MetricsCollector, logger Logger, defaultTTL time.Duration) *Decorator {
	if metrics == nil {
		metrics = &DefaultMetricsCollector{}
	}
	if logger == nil {
		logger = &DefaultLogger{}
	}
	return &Decorator{
		strategy:   strategy,
		metrics:    metrics,
		logger:     logger,
		defaultTTL: defaultTTL,
	}
}

// Get 获取数据，如果缓存不存在则调用fetchFunc获取
func (d *Decorator) Get(ctx context.Context, key string, fetchFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	if ttl == 0 {
		ttl = d.defaultTTL
	}

	start := time.Now()
	result, err := d.strategy.Get(ctx, key, fetchFunc, ttl)
	duration := time.Since(start)

	// 记录指标
	d.metrics.RecordCacheAccess(key, err == nil && result != nil, duration)

	// 记录日志
	if err != nil {
		d.logger.Error("Cache access error", "key", key, "error", err)
	} else if result == nil {
		d.logger.Info("Cache miss", "key", key, "duration", duration)
	} else {
		d.logger.Info("Cache hit", "key", key, "duration", duration)
	}

	return result, err
} 