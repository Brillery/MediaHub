package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// User 用户模型
type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// UserRepository 用户仓储
type UserRepository struct {
	// 这里应该有数据库连接，但为了示例简化，我们使用模拟数据
	cache *Decorator
}

// NewUserRepository 创建用户仓储
func NewUserRepository(factory *Factory) *UserRepository {
	// 创建缓存策略
	strategy := factory.NewStrategy("all")
	
	// 创建缓存装饰器
	decorator := NewDecorator(
		strategy,
		&DefaultMetricsCollector{},
		&DefaultLogger{},
		factory.config.DefaultTTL,
	)
	
	return &UserRepository{
		cache: decorator,
	}
}

// GetByID 根据ID获取用户
func (r *UserRepository) GetByID(ctx context.Context, userID int64) (*User, error) {
	// 构建缓存键
	cacheKey := fmt.Sprintf("user:%d", userID)
	
	// 定义从数据库获取数据的函数
	fetchFunc := func() (interface{}, error) {
		// 这里应该是从数据库查询，但为了示例，我们使用模拟数据
		if userID == 1 {
			return &User{
				ID:       1,
				Username: "user1",
				Email:    "user1@example.com",
			}, nil
		}
		// 模拟数据不存在的情况
		return nil, nil
	}
	
	// 使用缓存装饰器获取数据
	data, err := r.cache.Get(ctx, cacheKey, fetchFunc, 30*time.Minute)
	if err != nil {
		return nil, err
	}
	
	// 如果数据为空，返回nil
	if data == nil {
		return nil, nil
	}
	
	// 将数据转换为User类型
	userStr, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("invalid data type")
	}
	
	var user User
	err = json.Unmarshal([]byte(userStr), &user)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

// ExampleUsage 展示如何使用缓存脚手架
func ExampleUsage() {
	// 创建Redis配置
	config := &Config{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 5,
		MaxConnAge:   time.Hour,
		IdleTimeout:  time.Minute * 10,
		DefaultTTL:   time.Hour,
	}
	
	// 创建缓存工厂
	factory := NewFactory(config)
	defer factory.Close()
	
	// 创建用户仓储
	userRepo := NewUserRepository(factory)
	
	// 创建上下文
	ctx := context.Background()
	
	// 获取存在的用户
	user, err := userRepo.GetByID(ctx, 1)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	if user != nil {
		fmt.Printf("User found: %+v\n", user)
	} else {
		fmt.Println("User not found")
	}
	
	// 获取不存在的用户
	user, err = userRepo.GetByID(ctx, 999)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	if user != nil {
		fmt.Printf("User found: %+v\n", user)
	} else {
		fmt.Println("User not found")
	}
} 