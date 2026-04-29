package service

import (
	"context"
	"testing"

	"github.com/sonni-a/minibank/api/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestTransfer_InvalidAmount_ReturnsInvalidArgument(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.Transfer(context.Background(), &payment.TransferRequest{
		AmountMinor: 0,
		ToUserId:    2,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v (err=%v)", codes.InvalidArgument, status.Code(err), err)
	}
}

func TestDeposit_InvalidAmount_ReturnsInvalidArgument(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.Deposit(context.Background(), &payment.DepositRequest{
		AmountMinor: -1,
		UserId:      1,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected %v, got %v (err=%v)", codes.InvalidArgument, status.Code(err), err)
	}
}

func TestCreateAccount_WithoutMetadata_ReturnsUnauthenticated(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.CreateAccount(context.Background(), &payment.CreateAccountRequest{UserId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected %v, got %v (err=%v)", codes.Unauthenticated, status.Code(err), err)
	}
}
