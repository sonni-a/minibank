package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/sonni-a/minibank/api/auth"
	"github.com/sonni-a/minibank/pkg/jwt"
	pkgredis "github.com/sonni-a/minibank/pkg/redis"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthService struct {
	auth.UnimplementedAuthServiceServer
	db *sql.DB
}

func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

func (s *AuthService) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Error hashing password:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	_, err = s.db.Exec("INSERT INTO auth_users (email, password_hash) VALUES ($1, $2)", req.Email, string(hash))
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return nil, status.Errorf(codes.AlreadyExists, "email already registered")
		}
		log.Println("DB insert error:", err)
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
		log.Println("DB query error:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid password")
	}

	cachedToken, _ := pkgredis.RDB.Get(ctx, "auth:token:"+req.Email).Result()
	cachedRefresh, _ := pkgredis.RDB.Get(ctx, "auth:refresh:"+req.Email).Result()
	if cachedToken != "" && cachedRefresh != "" {
		return &auth.AuthResponse{
			Token:        cachedToken,
			RefreshToken: cachedRefresh,
			Message:      "Login successful (cached tokens)",
		}, nil
	}

	return s.generateAndCacheTokens(ctx, req.Email)
}

func (s *AuthService) RefreshToken(ctx context.Context, req *auth.RefreshTokenRequest) (*auth.AuthResponse, error) {
	email, err := jwt.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid refresh token")
	}

	cachedRefresh, _ := pkgredis.RDB.Get(ctx, "auth:refresh:"+email).Result()
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
		log.Println("DeleteAuthUser db error:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	rows, err := res.RowsAffected()
	if err != nil {
		log.Println("DeleteAuthUser rows affected error:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}
	if rows == 0 {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}

	_ = pkgredis.RDB.Del(ctx, "auth:token:"+req.Email, "auth:refresh:"+req.Email).Err()

	return &auth.DeleteAuthUserResponse{Message: "auth user deleted"}, nil
}

func (s *AuthService) generateAndCacheTokens(ctx context.Context, email string) (*auth.AuthResponse, error) {
	token, err := jwt.GenerateJWT(email)
	if err != nil {
		log.Println("JWT generation error:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	refreshToken, err := jwt.GenerateRefreshToken(email)
	if err != nil {
		log.Println("Refresh JWT generation error:", err)
		return nil, status.Errorf(codes.Internal, "internal server error")
	}

	err = pkgredis.RDB.Set(ctx, "auth:token:"+email, token, 15*time.Minute).Err()
	if err != nil {
		log.Println("Redis set access token error:", err)
	}

	err = pkgredis.RDB.Set(ctx, "auth:refresh:"+email, refreshToken, 7*24*time.Hour).Err()
	if err != nil {
		log.Println("Redis set refresh token error:", err)
	}

	return &auth.AuthResponse{
		Token:        token,
		RefreshToken: refreshToken,
		Message:      "Operation successful",
	}, nil
}
