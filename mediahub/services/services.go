package services

import (
	"context"
	"google.golang.org/grpc/metadata"
)

func AppendBearerTokenToContext(ctx context.Context, accessToken string) context.Context {
	md := metadata.Pairs("Authorization", "Bearer "+accessToken)
	return metadata.NewOutgoingContext(ctx, md)
}
