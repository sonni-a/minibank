package service

import (
	"context"
	"errors"

	"github.com/sonni-a/minibank/payment-service/internal/grpc/payment"
	"github.com/sonni-a/minibank/payment-service/internal/repository"
)

type PaymentService struct {
	repo *repository.PaymentRepository
	payment.UnimplementedPaymentServiceServer
}

func NewPaymentService(repo *repository.PaymentRepository) *PaymentService {
	return &PaymentService{repo: repo}
}

func (s *PaymentService) CreateAccount(ctx context.Context, req *payment.CreateAccountRequest) (*payment.AccountResponse, error) {
	err := s.repo.CreateAccount(req.UserId)
	if err != nil {
		return nil, errors.New("account already exists")
	}

	return &payment.AccountResponse{
		UserId:  req.UserId,
		Balance: 0,
	}, nil
}

func (s *PaymentService) GetBalance(ctx context.Context, req *payment.GetBalanceRequest) (*payment.BalanceResponse, error) {
	balance, err := s.repo.GetBalance(req.UserId)
	if err != nil {
		return nil, err
	}

	return &payment.BalanceResponse{
		Balance: balance,
	}, nil
}

func (s *PaymentService) Transfer(ctx context.Context, req *payment.TransferRequest) (*payment.TransferResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	if req.FromUserId == req.ToUserId {
		return nil, errors.New("cannot transfer to yourself")
	}

	err := s.repo.Transfer(req.FromUserId, req.ToUserId, req.Amount)
	if err != nil {
		return nil, err
	}

	return &payment.TransferResponse{
		Message: "transfer successful",
	}, nil
}

func (s *PaymentService) Deposit(ctx context.Context, req *payment.DepositRequest) (*payment.BalanceResponse, error) {
	if req.Amount <= 0 {
		return nil, errors.New("invalid amount")
	}
	err := s.repo.Deposit(req.UserId, req.Amount)
	if err != nil {
		return nil, err
	}

	balance, err := s.repo.GetBalance(req.UserId)
	if err != nil {
		return nil, err
	}

	return &payment.BalanceResponse{
		Balance: balance,
	}, nil
}
