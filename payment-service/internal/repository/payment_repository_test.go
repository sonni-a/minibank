package repository

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func setupRepo(t *testing.T) (*PaymentRepository, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return NewPaymentRepository(db), mock
}

func TestTransfer_InsufficientFunds_ReturnsError(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(100)))
	mock.ExpectRollback()

	err := repo.Transfer(1, 2, 200)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_Success_UpdatesBothAccounts(t *testing.T) {
	repo, mock := setupRepo(t)

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

	if err := repo.Transfer(1, 2, 100); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeposit_AccountNotFound_ReturnsError(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Deposit(42, 100)
	if !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_SenderNotFound_ReturnsError(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(500)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance - $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repo.Transfer(1, 2, 100)
	if !errors.Is(err, ErrSenderNotFound) {
		t.Fatalf("expected ErrSenderNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_RecipientNotFound_ReturnsError(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(500)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance - $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := repo.Transfer(1, 2, 100)
	if !errors.Is(err, ErrRecipientNotFound) {
		t.Fatalf("expected ErrRecipientNotFound, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestDeposit_Success(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.Deposit(1, 100); err != nil {
		t.Fatalf("Deposit: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestCreateAccount_Success(t *testing.T) {
	repo, mock := setupRepo(t)

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO accounts (user_id, balance) VALUES ($1, 0)")).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.CreateAccount(1); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetBalance(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo, mock := setupRepo(t)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1")).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(500)))

		balance, err := repo.GetBalance(1)
		if err != nil {
			t.Fatalf("GetBalance: %v", err)
		}
		if balance != 500 {
			t.Fatalf("balance = %d, want 500", balance)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sql expectations: %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo, mock := setupRepo(t)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1")).
			WithArgs(int64(42)).
			WillReturnError(sql.ErrNoRows)

		_, err := repo.GetBalance(42)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("expected sql.ErrNoRows, got %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet sql expectations: %v", err)
		}
	})
}
