package service

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/sonni-a/minibank/api/auth"
	"github.com/sonni-a/minibank/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testEmail = "user@example.com"
const testPassword = "password123"

func setJWTEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "test-refresh-secret")
}

func setupAuthService(t *testing.T) (*AuthService, sqlmock.Sqlmock, *miniredis.Miniredis) {
	t.Helper()
	setJWTEnv(t)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewAuthService(db, rdb), mock, mr
}

func expectRegisterInsert(mock sqlmock.Sqlmock, email string) {
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO auth_users (email, password_hash) VALUES ($1, $2)")).
		WithArgs(email, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
}

func TestRegister_EmptyFields_ReturnsInvalidArgument(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	_, err := svc.Register(context.Background(), &auth.RegisterRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestRegister_InvalidInput_ReturnsInvalidArgument(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	tests := []struct {
		name string
		req  *auth.RegisterRequest
	}{
		{"invalid email", &auth.RegisterRequest{Email: "not-an-email", Password: testPassword}},
		{"short password", &auth.RegisterRequest{Email: testEmail, Password: "short"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(context.Background(), tt.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
			}
		})
	}
}

func TestRegister_DuplicateEmail_ReturnsAlreadyExists(t *testing.T) {
	svc, mock, _ := setupAuthService(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO auth_users (email, password_hash) VALUES ($1, $2)")).
		WithArgs(testEmail, sqlmock.AnyArg()).
		WillReturnError(&pq.Error{Code: pgUniqueViolation})

	_, err := svc.Register(context.Background(), &auth.RegisterRequest{
		Email:    testEmail,
		Password: testPassword,
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRegister_Success_ReturnsTokensAndCachesThem(t *testing.T) {
	svc, mock, mr := setupAuthService(t)
	ctx := context.Background()

	expectRegisterInsert(mock, testEmail)

	resp, err := svc.Register(ctx, &auth.RegisterRequest{
		Email:    testEmail,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatal("expected non-empty access and refresh tokens")
	}

	if got, err := mr.Get("auth:token:" + testEmail); err != nil || got != resp.Token {
		t.Fatalf("cached access token mismatch: got=%q err=%v", got, err)
	}
	if got, err := mr.Get("auth:refresh:" + testEmail); err != nil || got != resp.RefreshToken {
		t.Fatalf("cached refresh token mismatch: got=%q err=%v", got, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestLogin_EmptyFields_ReturnsInvalidArgument(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	_, err := svc.Login(context.Background(), &auth.LoginRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestLogin_InvalidEmail_ReturnsInvalidArgument(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	_, err := svc.Login(context.Background(), &auth.LoginRequest{
		Email:    "bad-email",
		Password: testPassword,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestLogin_UserNotFound_ReturnsNotFound(t *testing.T) {
	svc, mock, _ := setupAuthService(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT password_hash FROM auth_users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnError(sql.ErrNoRows)

	_, err := svc.Login(context.Background(), &auth.LoginRequest{
		Email:    testEmail,
		Password: testPassword,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestLogin_InvalidPassword_ReturnsUnauthenticated(t *testing.T) {
	svc, mock, _ := setupAuthService(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("other-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT password_hash FROM auth_users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(hash)))

	_, err = svc.Login(context.Background(), &auth.LoginRequest{
		Email:    testEmail,
		Password: testPassword,
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestLogin_Success_ReturnsTokens(t *testing.T) {
	svc, mock, mr := setupAuthService(t)
	ctx := context.Background()

	hash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	mock.ExpectQuery(regexp.QuoteMeta("SELECT password_hash FROM auth_users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnRows(sqlmock.NewRows([]string{"password_hash"}).AddRow(string(hash)))

	resp, err := svc.Login(ctx, &auth.LoginRequest{
		Email:    testEmail,
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if got, err := mr.Get("auth:refresh:" + testEmail); err != nil || got != resp.RefreshToken {
		t.Fatalf("refresh token was not cached: got=%q err=%v", got, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestRefreshToken_InvalidToken_ReturnsUnauthenticated(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	_, err := svc.RefreshToken(context.Background(), &auth.RefreshTokenRequest{
		RefreshToken: "not-a-valid-token",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestRefreshToken_NotInCache_ReturnsUnauthenticated(t *testing.T) {
	svc, _, _ := setupAuthService(t)

	refreshToken, err := jwt.GenerateRefreshToken(testEmail)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	_, err = svc.RefreshToken(context.Background(), &auth.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestRefreshToken_Success_ReturnsNewTokens(t *testing.T) {
	svc, _, mr := setupAuthService(t)
	ctx := context.Background()

	oldRefresh, err := jwt.GenerateRefreshToken(testEmail)
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	mr.Set("auth:refresh:"+testEmail, oldRefresh)

	resp, err := svc.RefreshToken(ctx, &auth.RefreshTokenRequest{RefreshToken: oldRefresh})
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if resp.Token == "" || resp.RefreshToken == "" {
		t.Fatal("expected non-empty tokens")
	}
	if got, err := mr.Get("auth:refresh:" + testEmail); err != nil || got != resp.RefreshToken {
		t.Fatalf("refresh token was not cached: got=%q err=%v", got, err)
	}
}

func TestDeleteAuthUser_EmptyEmail_ReturnsInvalidArgument(t *testing.T) {
	svc := &AuthService{}

	_, err := svc.DeleteAuthUser(context.Background(), &auth.DeleteAuthUserRequest{Email: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestDeleteAuthUser_UserNotFound_ReturnsNotFound(t *testing.T) {
	svc, mock, _ := setupAuthService(t)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM auth_users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.DeleteAuthUser(context.Background(), &auth.DeleteAuthUserRequest{Email: testEmail})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeleteAuthUser_Success_DeletesUserAndCache(t *testing.T) {
	svc, mock, mr := setupAuthService(t)
	ctx := context.Background()

	mr.Set("auth:token:"+testEmail, "access")
	mr.Set("auth:refresh:"+testEmail, "refresh")

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM auth_users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := svc.DeleteAuthUser(ctx, &auth.DeleteAuthUserRequest{Email: testEmail})
	if err != nil {
		t.Fatalf("DeleteAuthUser: %v", err)
	}
	if resp.Message != "auth user deleted" {
		t.Fatalf("message = %q, want auth user deleted", resp.Message)
	}
	if mr.Exists("auth:token:" + testEmail) {
		t.Fatal("access token cache was not deleted")
	}
	if mr.Exists("auth:refresh:" + testEmail) {
		t.Fatal("refresh token cache was not deleted")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
