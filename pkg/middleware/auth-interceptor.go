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

type ctxKey string

const UserEmailKey ctxKey = "user_email"

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
			return nil, status.Errorf(codes.Unauthenticated, "metadata not provided")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "authorization header missing")
		}

		token := strings.TrimPrefix(authHeader[0], "Bearer ")

		email, err := jwt.ValidateJWT(token)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid access token")
		}

		newCtx := context.WithValue(ctx, UserEmailKey, email)

		return handler(newCtx, req)
	}
}
