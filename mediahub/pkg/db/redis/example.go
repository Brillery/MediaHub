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

// UserService 用户服务
type UserService struct {
	cacheService *CacheService
	// 这里应该有数据库连接，但为了示例简化，我们使用模拟数据
}

// NewUserService 创建用户服务
func NewUserService() *UserService {
	return &UserService{
		cacheService: NewCacheService(),
	}
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(ctx context.Context, userID int64) (*User, error) {
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
	
	// 使用综合保护方法获取数据
	data, err := s.cacheService.GetWithAllProtections(ctx, cacheKey, 30*time.Minute, fetchFunc)
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

// ExampleUsage 展示如何使用缓存保护方法
func ExampleUsage() {
	ctx := context.Background()
	userService := NewUserService()
	
	// 获取存在的用户
	user, err := userService.GetUserByID(ctx, 1)
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
	user, err = userService.GetUserByID(ctx, 999)
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