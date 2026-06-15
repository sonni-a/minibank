package service

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/sonni-a/minibank/api/payment"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type stubUserClient struct {
	userID int64
}

func (s *stubUserClient) CreateUser(context.Context, *user.CreateUserRequest, ...grpc.CallOption) (*user.UserResponse, error) {
	panic("not implemented")
}

func (s *stubUserClient) GetUser(context.Context, *user.GetUserRequest, ...grpc.CallOption) (*user.UserResponse, error) {
	panic("not implemented")
}

func (s *stubUserClient) GetMyUser(context.Context, *user.GetMyUserRequest, ...grpc.CallOption) (*user.UserResponse, error) {
	return &user.UserResponse{Id: s.userID}, nil
}

func (s *stubUserClient) UpdateUser(context.Context, *user.UpdateUserRequest, ...grpc.CallOption) (*user.UserResponse, error) {
	panic("not implemented")
}

func (s *stubUserClient) DeleteUser(context.Context, *user.DeleteUserRequest, ...grpc.CallOption) (*user.DeleteUserResponse, error) {
	panic("not implemented")
}

func setupPaymentService(t *testing.T, userID int64) (*PaymentService, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo := repository.NewPaymentRepository(db)
	return NewPaymentService(repo, &stubUserClient{userID: userID}), mock
}

func ctxWithMetadata() context.Context {
	md := metadata.Pairs("authorization", "Bearer test")
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestCreateAccount_WithoutMetadata_ReturnsUnauthenticated(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.CreateAccount(context.Background(), &payment.CreateAccountRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestCreateAccount_WithUserID_Success(t *testing.T) {
	svc, mock := setupPaymentService(t, 0)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO accounts (user_id, balance) VALUES ($1, 0)")).
		WithArgs(int64(5)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	resp, err := svc.CreateAccount(context.Background(), &payment.CreateAccountRequest{UserId: 5})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if resp.UserId != 5 || resp.BalanceMinor != 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetBalance_WithoutMetadata_ReturnsUnauthenticated(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.GetBalance(context.Background(), &payment.GetBalanceRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestGetBalance_AccountNotFound_ReturnsNotFound(t *testing.T) {
	svc, mock := setupPaymentService(t, 1)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1")).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	_, err := svc.GetBalance(ctxWithMetadata(), &payment.GetBalanceRequest{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_InvalidAmount_ReturnsInvalidArgument(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.Transfer(context.Background(), &payment.TransferRequest{
		AmountMinor: 0,
		ToUserId:    2,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestTransfer_InsufficientFunds_ReturnsFailedPrecondition(t *testing.T) {
	svc, mock := setupPaymentService(t, 1)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(100)))
	mock.ExpectRollback()

	_, err := svc.Transfer(ctxWithMetadata(), &payment.TransferRequest{
		AmountMinor: 200,
		ToUserId:    2,
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_Success_ReturnsMessage(t *testing.T) {
	svc, mock := setupPaymentService(t, 1)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(500)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance - $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	resp, err := svc.Transfer(ctxWithMetadata(), &payment.TransferRequest{
		AmountMinor: 100,
		ToUserId:    2,
	})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if resp.Message != "transfer successful" {
		t.Fatalf("message = %q, want transfer successful", resp.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeposit_InvalidAmount_ReturnsInvalidArgument(t *testing.T) {
	svc := &PaymentService{}

	_, err := svc.Deposit(context.Background(), &payment.DepositRequest{AmountMinor: -1})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestDeposit_AccountNotFound_ReturnsNotFound(t *testing.T) {
	svc, mock := setupPaymentService(t, 1)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.Deposit(ctxWithMetadata(), &payment.DepositRequest{AmountMinor: 100})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeposit_Success_ReturnsBalance(t *testing.T) {
	svc, mock := setupPaymentService(t, 1)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(100)))

	resp, err := svc.Deposit(ctxWithMetadata(), &payment.DepositRequest{AmountMinor: 100})
	if err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if resp.BalanceMinor != 100 {
		t.Fatalf("balance = %d, want 100", resp.BalanceMinor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
