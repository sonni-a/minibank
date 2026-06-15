package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/sonni-a/minibank/api/auth"
	"github.com/sonni-a/minibank/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const pgUniqueViolation = "23505"

type AuthService struct {
	auth.UnimplementedAuthServiceServer
	db    *sql.DB
	cache *redis.Client
}

func NewAuthService(db *sql.DB, cache *redis.Client) *AuthService {
	return &AuthService{db: db, cache: cache}
}

func (s *AuthService) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("failed to hash password", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	_, err = s.db.Exec("INSERT INTO auth_users (email, password_hash) VALUES ($1, $2)", req.Email, string(hash))
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == pgUniqueViolation {
			return nil, status.Errorf(codes.AlreadyExists, "email already registered")
		}
		slog.Error("db insert failed", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	return s.generateAndCacheTokens(ctx, req.Email)
}

func (s *AuthService) Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error) {
	var hash string
	row := s.db.QueryRow("SELECT password_hash FROM auth_users WHERE email=$1", req.Email)
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}
		slog.Error("db query failed", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid password")
	}

	return s.generateAndCacheTokens(ctx, req.Email)
}

func (s *AuthService) RefreshToken(ctx context.Context, req *auth.RefreshTokenRequest) (*auth.AuthResponse, error) {
	email, err := jwt.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid refresh token")
	}

	cachedRefresh, _ := s.cache.Get(ctx, "auth:refresh:"+email).Result()
	if cachedRefresh == "" || cachedRefresh != req.RefreshToken {
		return nil, status.Errorf(codes.Unauthenticated, "refresh token expired or not found")
	}

	return s.generateAndCacheTokens(ctx, email)
}

func (s *AuthService) DeleteAuthUser(ctx context.Context, req *auth.DeleteAuthUserRequest) (*auth.DeleteAuthUserResponse, error) {
	if req.Email == "" {
		return nil, status.Errorf(codes.InvalidArgument, "email is required")
	}

	res, err := s.db.ExecContext(ctx, "DELETE FROM auth_users WHERE email=$1", req.Email)
	if err != nil {
		slog.Error("DeleteAuthUser db error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		slog.Error("DeleteAuthUser rows affected error", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	_ = s.cache.Del(ctx, "auth:token:"+req.Email, "auth:refresh:"+req.Email).Err()

	return &auth.DeleteAuthUserResponse{Message: "auth user deleted"}, nil
}

func (s *AuthService) generateAndCacheTokens(ctx context.Context, email string) (*auth.AuthResponse, error) {
	token, err := jwt.GenerateJWT(email)
	if err != nil {
		slog.Error("failed to generate JWT", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	refreshToken, err := jwt.GenerateRefreshToken(email)
	if err != nil {
		slog.Error("failed to generate refresh JWT", "error", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	err = s.cache.Set(ctx, "auth:token:"+email, token, 15*time.Minute).Err()
	if err != nil {
		slog.Warn("failed to cache access token", "error", err)
	}

	err = s.cache.Set(ctx, "auth:refresh:"+email, refreshToken, 7*24*time.Hour).Err()
	if err != nil {
		slog.Warn("failed to cache refresh token", "error", err)
	}

	return &auth.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		Message:      "Operation successful",
	}, nil
}
