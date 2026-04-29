package service

import (
	"context"
	"testing"

	"github.com/sonni-a/minibank/api/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetMyUser_WithoutAuthContext_ReturnsUnauthenticated(t *testing.T) {
	svc := &UserService{}

	_, err := svc.GetMyUser(context.Background(), &user.GetMyUserRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected %v, got %v (err=%v)", codes.Unauthenticated, status.Code(err), err)
	}
}
