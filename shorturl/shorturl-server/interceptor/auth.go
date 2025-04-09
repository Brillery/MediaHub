package interceptor

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"shorturl/pkg/config"
	"shorturl/pkg/zerror"
	"strings"
)

func UnaryAuthInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	// todo 如果做了健康检查，要调过健康检查
	fmt.Println(info.FullMethod)
	if info.FullMethod != "/grpc.health.v1.Health/Check" {
		err = oauth2Valid(ctx)
		if err != nil {
			return nil, err
		}
	}

	return handler(ctx, req)
}

func StreamAuthInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := oauth2Valid(ss.Context())
	if err != nil {
		return err
	}

	return handler(srv, ss)
}

func oauth2Valid(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return zerror.NewByMsg("元数据获取失败，身份认证失败")
	}
	//authorization := md["Authorization"] 同
	authorization := md.Get("Authorization")
	if len(authorization) < 1 {
		return zerror.NewByMsg("元数据获取失败，身份认证失败")
	}

	token := strings.TrimPrefix(authorization[0], "Bearer ")
	cnf := config.GetConfig()
	if cnf.Server.AccessToken != token {
		return zerror.NewByMsg("身份认证失败")
	}

	return nil
}
