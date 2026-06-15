package middleware

import (
	"context"
	"testing"

	"github.com/sonni-a/minibank/pkg/jwt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testPublicMethod = "/auth.AuthService/Register"

func setJWTEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "test-refresh-secret")
}

func TestAuthInterceptor_PublicMethodSkipsAuth(t *testing.T) {
	interceptor := AuthInterceptor(testPublicMethod)

	called := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: testPublicMethod},
		handler,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called for public method")
	}
	if resp != "ok" {
		t.Fatalf("resp = %v, want ok", resp)
	}
}

func TestAuthInterceptor_NoMetadata_ReturnsUnauthenticated(t *testing.T) {
	setJWTEnv(t)
	interceptor := AuthInterceptor()

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/private/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestAuthInterceptor_MissingAuthHeader_ReturnsUnauthenticated(t *testing.T) {
	setJWTEnv(t)
	interceptor := AuthInterceptor()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("other", "value"))
	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/private/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestAuthInterceptor_InvalidToken_ReturnsUnauthenticated(t *testing.T) {
	setJWTEnv(t)
	interceptor := AuthInterceptor()

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer invalid-token"),
	)
	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/private/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestAuthInterceptor_ValidToken_SetsEmailInContext(t *testing.T) {
	setJWTEnv(t)

	token, err := jwt.GenerateJWT("user@example.com")
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	interceptor := AuthInterceptor()
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer "+token),
	)

	var gotEmail string
	_, err = interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{FullMethod: "/private/Method"},
		func(ctx context.Context, req interface{}) (interface{}, error) {
			gotEmail, _ = ctx.Value(UserEmailKey).(string)
			return nil, nil
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotEmail != "user@example.com" {
		t.Fatalf("context email = %q, want user@example.com", gotEmail)
	}
}
