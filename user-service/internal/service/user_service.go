package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/user-service/internal/models"
)

type UserService struct {
	db *sql.DB
	user.UnimplementedUserServiceServer
}

func NewUserService(db *sql.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var id int64
	err := s.db.QueryRowContext(ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		req.Name, req.Email).Scan(&id)
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return nil, errors.New("email already exists")
		}
		log.Println("CreateUser db error:", err)
		return nil, errors.New("internal server error")
	}

	return &user.UserResponse{Id: id, Name: req.Name, Email: req.Email}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var u models.User
	row := s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id=$1", req.Id)
	if err := row.Scan(&u.ID, &u.Name, &u.Email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		log.Println("GetUser db error:", err)
		return nil, errors.New("internal server error")
	}

	return &user.UserResponse{Id: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, "UPDATE users SET name=$1, email=$2 WHERE id=$3", req.Name, req.Email, req.Id)
	if err != nil {
		log.Println("UpdateUser db error:", err)
		return nil, errors.New("internal server error")
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, errors.New("user not found")
	}

	return &user.UserResponse{Id: req.Id, Name: req.Name, Email: req.Email}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *user.DeleteUserRequest) (*user.DeleteUserResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	res, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", req.Id)
	if err != nil {
		log.Println("DeleteUser db error:", err)
		return nil, errors.New("internal server error")
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, errors.New("user not found")
	}

	return &user.DeleteUserResponse{Message: "User deleted"}, nil
}
