package middleware

import (
	"context"
	"strings"

	"github.com/sonni-a/minibank/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func AuthInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		if strings.Contains(info.FullMethod, "CreateUser") {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Errorf(codes.Unauthenticated, "no metadata provided")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "no authorization header")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")

		_, err := jwt.ValidateJWT(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token")
		}

		return handler(ctx, req)
	}
}
