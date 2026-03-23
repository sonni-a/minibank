package service

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/lib/pq"
	"github.com/sonni-a/minibank/auth-service/internal/grpc/auth"
	"github.com/sonni-a/minibank/pkg/jwt"
	pkgredis "github.com/sonni-a/minibank/pkg/redis"
	"golang.org/x/crypto/bcrypt"
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
		return nil, errors.New("internal server error")
	}

	_, err = s.db.Exec("INSERT INTO auth_users (email, password_hash) VALUES ($1, $2)", req.Email, string(hash))
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
			return nil, errors.New("email already registered")
		}
		log.Println("DB insert error:", err)
		return nil, errors.New("internal server error")
	}

	return s.generateAndCacheToken(ctx, req.Email)
}

func (s *AuthService) Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error) {
	var hash string
	row := s.db.QueryRow("SELECT password_hash FROM auth_users WHERE email=$1", req.Email)
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		log.Println("DB query error:", err)
		return nil, errors.New("internal server error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid password")
	}

	cachedToken, err := pkgredis.RDB.Get(ctx, "auth:token:"+req.Email).Result()
	if err == nil && cachedToken != "" {
		return &auth.AuthResponse{
			Token:        cachedToken,
			RefreshToken: "",
			Message:      "Login successful (cached token)",
		}, nil
	}

	return s.generateAndCacheToken(ctx, req.Email)
}

func (s *AuthService) generateAndCacheToken(ctx context.Context, email string) (*auth.AuthResponse, error) {
	token, err := jwt.GenerateJWT(email)
	if err != nil {
		log.Println("JWT generation error:", err)
		return nil, errors.New("internal server error")
	}

	err = pkgredis.RDB.Set(ctx, "auth:token:"+email, token, time.Hour).Err()
	if err != nil {
		log.Println("Redis set error:", err)
	}

	return &auth.AuthResponse{
		Token:        token,
		RefreshToken: "",
		Message:      "Operation successful",
	}, nil
}
