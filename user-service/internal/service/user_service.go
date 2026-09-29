package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/lib/pq"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/pkg/middleware"
	"github.com/sonni-a/minibank/pkg/validate"
	"github.com/sonni-a/minibank/user-service/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const pgUniqueViolation = "23505"

type UserService struct {
	db *sql.DB
	user.UnimplementedUserServiceServer
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func emailFromContext(ctx context.Context) (string, error) {
	email, ok := ctx.Value(middleware.UserEmailKey).(string)
	if !ok || email == "" {
		return "", status.Errorf(codes.Unauthenticated, "unauthenticated")
	}
	return email, nil
}

func (s *UserService) ownUser(ctx context.Context, id int64) (models.User, error) {
	email, err := emailFromContext(ctx)
	if err != nil {
		return models.User{}, err
	}

	var u models.User
	err = s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id=$1", id).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, status.Errorf(codes.NotFound, "user not found")
		}
		slog.Error("ownUser db error", "error", err)
		return models.User{}, status.Errorf(codes.Internal, "internal server error")
	}
	if u.Email != email {
		return models.User{}, status.Errorf(codes.PermissionDenied, "user_id does not match authenticated user")
	}
	return u, nil
}

func (s *UserService) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.UserResponse, error) {
	if req.Name == "" || req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name and email are required")
	}
	if !validate.Email(req.Email) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email format")
	}
	tokenEmail, err := emailFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.Email != tokenEmail {
		return nil, status.Errorf(codes.PermissionDenied, "email does not match authenticated user")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var id int64
	err = s.db.QueryRowContext(ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		req.Name, req.Email).Scan(&id)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == pgUniqueViolation {
			return nil, status.Errorf(codes.AlreadyExists, "email already exists")
		}
		slog.Error("CreateUser db error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &user.UserResponse{Id: id, Name: req.Name, Email: req.Email}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	u, err := s.ownUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &user.UserResponse{Id: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *UserService) GetMyUser(ctx context.Context, req *user.GetMyUserRequest) (*user.UserResponse, error) {
	_ = req
	email, err := emailFromContext(ctx)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var u models.User
	err = s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE email=$1", email).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		slog.Error("GetMyUser db error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return &user.UserResponse{Id: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UserResponse, error) {
	if req.Name == "" || req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name and email are required")
	}
	if !validate.Email(req.Email) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid email format")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if _, err := s.ownUser(ctx, req.Id); err != nil {
		return nil, err
	}

	res, err := s.db.ExecContext(ctx, "UPDATE users SET name=$1, email=$2 WHERE id=$3", req.Name, req.Email, req.Id)
	if err != nil {
		slog.Error("UpdateUser db error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		slog.Error("UpdateUser rows affected error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &user.UserResponse{Id: req.Id, Name: req.Name, Email: req.Email}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *user.DeleteUserRequest) (*user.DeleteUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if _, err := s.ownUser(ctx, req.Id); err != nil {
		return nil, err
	}

	res, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", req.Id)
	if err != nil {
		slog.Error("DeleteUser db error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		slog.Error("DeleteUser rows affected error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	return &user.DeleteUserResponse{Message: "User deleted"}, nil
}
