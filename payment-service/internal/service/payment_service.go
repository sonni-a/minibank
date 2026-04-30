package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sonni-a/minibank/api/payment"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type PaymentService struct {
	repo *repository.PaymentRepository
	user user.UserServiceClient
	payment.UnimplementedPaymentServiceServer
}

func NewPaymentService(repo *repository.PaymentRepository, userClient user.UserServiceClient) *PaymentService {
	return &PaymentService{repo: repo, user: userClient}
}

// callerUserID resolves the authenticated user's id via user-service (JWT email → users.id)
func (s *PaymentService) callerUserID(ctx context.Context) (int64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.Unauthenticated, "metadata not provided")
	}
	outCtx := metadata.NewOutgoingContext(ctx, md)

	resp, err := s.user.GetMyUser(outCtx, &user.GetMyUserRequest{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

func (s *PaymentService) CreateAccount(ctx context.Context, req *payment.CreateAccountRequest) (*payment.AccountResponse, error) {
	var myID int64
	var err error
	if req.UserId > 0 {
		// Gateway registration flow passes a concrete user_id; avoid an extra user-service round-trip.
		myID = req.UserId
	} else {
		myID, err = s.callerUserID(ctx)
		if err != nil {
			return nil, err
		}
	}

	err = s.repo.CreateAccount(myID)
	if err != nil {
		return nil, status.Errorf(codes.AlreadyExists, "account already exists")
	}

	return &payment.AccountResponse{
		UserId:       myID,
		BalanceMinor: 0,
	}, nil
}

func (s *PaymentService) GetBalance(ctx context.Context, req *payment.GetBalanceRequest) (*payment.BalanceResponse, error) {
	myID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserId != 0 && req.UserId != myID {
		return nil, status.Errorf(codes.PermissionDenied, "user_id does not match authenticated user")
	}

	balance, err := s.repo.GetBalance(myID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "account not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &payment.BalanceResponse{
		BalanceMinor: balance,
	}, nil
}

func (s *PaymentService) Transfer(ctx context.Context, req *payment.TransferRequest) (*payment.TransferResponse, error) {
	if req.AmountMinor <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount")
	}

	myID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.FromUserId != 0 && req.FromUserId != myID {
		return nil, status.Errorf(codes.PermissionDenied, "from_user_id does not match authenticated user")
	}
	if myID == req.ToUserId {
		return nil, status.Errorf(codes.InvalidArgument, "cannot transfer to yourself")
	}

	err = s.repo.Transfer(myID, req.ToUserId, req.AmountMinor)
	if err != nil {
		switch err.Error() {
		case "insufficient funds":
			return nil, status.Errorf(codes.FailedPrecondition, "insufficient funds")
		case "debit failed: sender account missing", "recipient account not found":
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			if errors.Is(err, sql.ErrNoRows) {
				return nil, status.Errorf(codes.NotFound, "account not found")
			}
			return nil, status.Errorf(codes.Internal, "internal server error")
		}
	}

	return &payment.TransferResponse{
		Message: "transfer successful",
	}, nil
}

func (s *PaymentService) Deposit(ctx context.Context, req *payment.DepositRequest) (*payment.BalanceResponse, error) {
	if req.AmountMinor <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid amount")
	}

	myID, err := s.callerUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserId != 0 && req.UserId != myID {
		return nil, status.Errorf(codes.PermissionDenied, "user_id does not match authenticated user")
	}

	err = s.repo.Deposit(myID, req.AmountMinor)
	if err != nil {
		if err.Error() == "account not found" {
			return nil, status.Errorf(codes.NotFound, "account not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	balance, err := s.repo.GetBalance(myID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "account not found")
		}
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &payment.BalanceResponse{
		BalanceMinor: balance,
	}, nil
}
