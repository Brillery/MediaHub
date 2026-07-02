// Package shorturl 负责把 mediahub 上传链路接入内部 shorturl gRPC 服务。
//
// 模块职责：
// - 输入：全局配置中的 shorturl 地址和访问令牌。
// - 输出：全局复用的 gRPC 客户端连接池，供上传控制器创建短链。
// - 状态边界：只维护当前进程内的连接池单例，不缓存短链业务数据。
// - 外部依赖：grpc_client_pool、gRPC insecure transport、配置中心和日志模块。
// - 非职责：不负责短链生成、短链鉴权、上传文件校验或 HTTP 响应处理。
package shorturl

import (
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/pkg/grpc_client_pool"
	"enterprise-project1-mediahub/mediahub/pkg/log"
	"enterprise-project1-mediahub/mediahub/pkg/zerror"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"sync"
)

// pool 用于存储 gRPC 客户端连接，以实现连接复用
var pool grpc_client_pool.ClientPool

// once 确保在全局范围内只执行一次客户端连接池的初始化
var once sync.Once

// NewShortUrlClientPool 返回一个 gRPC 客户端连接池
// 该函数确保在程序的任何地方只初始化一次连接池
func NewShortUrlClientPool() grpc_client_pool.ClientPool {
	var err error
	// 如果连接池已经初始化，则直接返回
	if pool != nil {
		return pool
	}

	// 使用 sync.Once 确保连接池只被初始化一次
	once.Do(func() {
		// 获取配置信息
		cnf := config.GetConfig()
		// mediahub 主服务使用固定容量连接池，避免上传高峰时每个请求重复 Dial。
		pool, err = grpc_client_pool.NewClientCusPool(cnf.DependOn.ShortUrl.Address, 4, grpc.WithTransportCredentials(insecure.NewCredentials()))
		// 如果初始化过程中出现错误，记录错误日志
		if err != nil {
			log.Error(zerror.NewByErr(err))
		}
	})
	// 返回初始化后的 gRPC 客户端连接池
	return pool
}
