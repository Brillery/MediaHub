package proxy

import (
	"context"
	"enterprise-project1-mediahub/shorturl-proxy/pkg/config"
	"enterprise-project1-mediahub/shorturl-proxy/pkg/log"
	"enterprise-project1-mediahub/shorturl-proxy/services"
	"enterprise-project1-mediahub/shorturl-proxy/services/shorturl"
	"enterprise-project1-mediahub/shorturl-proxy/services/shorturl/proto"
	"github.com/gin-gonic/gin"
	"net/http"
)

// Proxy 结构体表示短链接代理服务的核心组件，负责处理短链接到原始URL的重定向请求。
// 字段：
// - config: 代理服务的配置信息
// - log: 日志记录接口
type Proxy struct {
	config *config.Config
	log    log.ILogger
}

// NewProxy 创建一个新的Proxy实例。
// 参数:
// - cnf *config.Config: 代理服务的配置信息
// - log log.ILogger: 日志记录接口
// 返回:
// - *Proxy: 新创建的Proxy实例
func NewProxy(cnf *config.Config, log log.ILogger) *Proxy {
	return &Proxy{
		config: cnf,
		log:    log,
	}
}

// PublicProxy 处理公共短链接的重定向请求。
// 参数:
// - ctx *gin.Context: Gin框架的HTTP请求上下文
// 该方法将isPublic参数设置为true并调用redirection方法
func (p *Proxy) PublicProxy(ctx *gin.Context) {
	p.redirection(ctx, true)
}

// UserProxy 处理用户专属短链接的重定向请求。
// 参数:
// - ctx *gin.Context: Gin框架的HTTP请求上下文
// 该方法将isPublic参数设置为false并调用redirection方法
func (p *Proxy) UserProxy(ctx *gin.Context) {
	p.redirection(ctx, false)
}

// redirection 处理短链接到原始URL的重定向逻辑。
// 参数:
// - ctx *gin.Context: Gin框架的HTTP请求上下文
// - isPublic bool: 标识短链接类型（true为公共，false为用户专属）
// 返回:
// - 无
// 流程:
// 1. 从URL路径提取短链接标识符
// 2. 调用服务获取原始URL
// 3. 处理错误并返回500状态码
// 4. 成功时执行HTTP重定向
func (p *Proxy) redirection(ctx *gin.Context, isPublic bool) {
	shortKey := ctx.Param("short_key")
	originalUrl, err := p.getOriginalUrl(shortKey, isPublic)

	// 错误处理模块
	if err != nil {
		p.log.Error(err)
		ctx.JSON(http.StatusInternalServerError, nil)
		return
	}

	// 成功重定向模块
	ctx.Redirect(http.StatusFound, originalUrl)
}

// getOriginalUrl 根据短链接键和类型获取原始URL。
// 参数:
// - shortKey string: 短链接唯一标识符
// - isPublic bool: 短链接类型标识（true为公共，false为用户专属）
// 返回:
// - string: 原始URL地址
// - error: 操作错误信息（如果发生）
// 流程:
// 1. 获取gRPC客户端连接池
// 2. 创建带访问令牌的gRPC上下文
// 3. 调用短链接服务的gRPC接口
// 4. 处理服务响应并返回结果
func (p *Proxy) getOriginalUrl(shortKey string, isPublic bool) (string, error) {
	// 客户端池管理模块
	pool := shorturl.NewShortUrlClientPool()
	conn := pool.Get()
	defer pool.Put(conn)

	// gRPC客户端初始化模块
	client := proto.NewShortUrlClient(conn)

	// 上下文构建模块
	ctx := services.AppendBearerTokenToContext(
		context.Background(),
		p.config.DependOn.ShortUrl.AccessToken,
	)

	// 请求参数构建模块
	req := &proto.ShortKey{
		Key:      shortKey,
		UserID:   0, // 当前实现未使用用户ID参数
		IsPublic: isPublic,
	}

	// gRPC调用模块
	rs, err := client.GetOriginalUrl(ctx, req)
	if err != nil {
		return "", err
	}

	return rs.Url, nil // 返回服务响应中的原始URL
}
