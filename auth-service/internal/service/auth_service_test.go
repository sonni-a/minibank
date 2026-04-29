package service

import (
	"context"
	"testing"

	"github.com/sonni-a/minibank/api/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDeleteAuthUser_EmptyEmail_ReturnsInvalidArgument(t *testing.T) {
	svc := &AuthService{}

	_, err := svc.DeleteAuthUser(context.Background(), &auth.DeleteAuthUserRequest{Email: ""})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v (err=%v)", codes.InvalidArgument, status.Code(err), err)
	}
}
