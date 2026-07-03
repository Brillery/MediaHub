package controller

import (
	"context"
	"enterprise-project1-mediahub/mediahub/pkg/config"
	"enterprise-project1-mediahub/mediahub/services"
	"enterprise-project1-mediahub/mediahub/services/shorturl"
	"enterprise-project1-mediahub/mediahub/services/shorturl/proto"
)

// ShortURLGenerator 负责把对象存储 URL 转成最终返回给前端的短链。
//
// 输入是对象存储返回的原始 URL、鉴权用户 ID 和公私有标记；输出是 shorturl 服务返回的短链。
// 该接口只描述上传控制器需要的能力，不负责图片校验、对象存储上传或 HTTP 响应。
type ShortURLGenerator interface {
	Generate(ctx context.Context, originalURL string, userID int64, isPublic bool) (string, error)
}

// grpcShortURLGenerator 是生产环境的 shorturl gRPC 适配器。
//
// 它复用 services/shorturl 中的进程内连接池，并通过 outgoing context 附加 shorturl 服务访问令牌。
// 状态边界：不缓存短链业务结果，不持有请求状态；连接复用由 shorturl.NewShortUrlClientPool 维护。
type grpcShortURLGenerator struct {
	accessToken string
}

func newGRPCShortURLGenerator(cnf *config.Config) ShortURLGenerator {
	accessToken := ""
	if cnf != nil {
		accessToken = cnf.DependOn.ShortUrl.AccessToken
	}
	return &grpcShortURLGenerator{accessToken: accessToken}
}

// Generate 调用 shorturl gRPC 服务生成短链。
//
// 错误直接返回给上传控制器，由控制器记录日志并转换成 HTTP 500；本层不吞错误，避免上传成功但短链失败时前端误以为流程完成。
func (g *grpcShortURLGenerator) Generate(ctx context.Context, originalURL string, userID int64, isPublic bool) (string, error) {
	shortPool := shorturl.NewShortUrlClientPool()
	clientConn := shortPool.Get()
	defer shortPool.Put(clientConn)

	client := proto.NewShortUrlClient(clientConn)
	outGoingCtx := services.AppendBearerTokenToContext(ctx, g.accessToken)
	outUrl, err := client.GetShortUrl(outGoingCtx, &proto.Url{
		Url:      originalURL,
		UserID:   userID,
		IsPublic: isPublic,
	})
	if err != nil {
		return "", err
	}
	return outUrl.Url, nil
}
