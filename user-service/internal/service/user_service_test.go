package service

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/sonni-a/minibank/api/user"
	"github.com/sonni-a/minibank/pkg/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testName = "Alice"
const testEmail = "user@example.com"

func setupUserService(t *testing.T) (*UserService, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return NewUserService(db), mock
}

func ctxWithEmail(email string) context.Context {
	return context.WithValue(context.Background(), middleware.UserEmailKey, email)
}

func TestCreateUser_EmptyFields_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := setupUserService(t)

	_, err := svc.CreateUser(context.Background(), &user.CreateUserRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestCreateUser_InvalidEmail_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := setupUserService(t)

	_, err := svc.CreateUser(context.Background(), &user.CreateUserRequest{
		Name:  testName,
		Email: "not-an-email",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
	}
}

func TestCreateUser_DuplicateEmail_ReturnsAlreadyExists(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id")).
		WithArgs(testName, testEmail).
		WillReturnError(&pq.Error{Code: pgUniqueViolation})

	_, err := svc.CreateUser(context.Background(), &user.CreateUserRequest{
		Name:  testName,
		Email: testEmail,
	})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUser_NotFound_ReturnsNotFound(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, email FROM users WHERE id=$1")).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)

	_, err := svc.GetUser(context.Background(), &user.GetUserRequest{Id: 42})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUser_Success_ReturnsUser(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, email FROM users WHERE id=$1")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(1), testName, testEmail))

	resp, err := svc.GetUser(context.Background(), &user.GetUserRequest{Id: 1})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if resp.Id != 1 || resp.Name != testName || resp.Email != testEmail {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetMyUser_WithoutAuthContext_ReturnsUnauthenticated(t *testing.T) {
	svc := &UserService{}

	_, err := svc.GetMyUser(context.Background(), &user.GetMyUserRequest{})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v (err=%v)", status.Code(err), err)
	}
}

func TestGetMyUser_Success_ReturnsUser(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, email FROM users WHERE email=$1")).
		WithArgs(testEmail).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "email"}).AddRow(int64(1), testName, testEmail))

	resp, err := svc.GetMyUser(ctxWithEmail(testEmail), &user.GetMyUserRequest{})
	if err != nil {
		t.Fatalf("GetMyUser: %v", err)
	}
	if resp.Id != 1 || resp.Email != testEmail {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestUpdateUser_InvalidInput_ReturnsInvalidArgument(t *testing.T) {
	svc, _ := setupUserService(t)

	tests := []struct {
		name string
		req  *user.UpdateUserRequest
	}{
		{"empty fields", &user.UpdateUserRequest{Id: 1}},
		{"invalid email", &user.UpdateUserRequest{Id: 1, Name: testName, Email: "bad-email"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateUser(context.Background(), tt.req)
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("expected InvalidArgument, got %v (err=%v)", status.Code(err), err)
			}
		})
	}
}

func TestUpdateUser_NotFound_ReturnsNotFound(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name=$1, email=$2 WHERE id=$3")).
		WithArgs(testName, testEmail, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.UpdateUser(context.Background(), &user.UpdateUserRequest{
		Id:    42,
		Name:  testName,
		Email: testEmail,
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeleteUser_NotFound_ReturnsNotFound(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM users WHERE id=$1")).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := svc.DeleteUser(context.Background(), &user.DeleteUserRequest{Id: 42})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v (err=%v)", status.Code(err), err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeleteUser_Success_ReturnsMessage(t *testing.T) {
	svc, mock := setupUserService(t)

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM users WHERE id=$1")).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := svc.DeleteUser(context.Background(), &user.DeleteUserRequest{Id: 1})
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if resp.Message != "User deleted" {
		t.Fatalf("message = %q, want User deleted", resp.Message)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
