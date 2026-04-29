package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTransfer_InsufficientFunds_ReturnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE")).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(int64(100)))
	mock.ExpectRollback()

	err = repo.Transfer(1, 2, 200)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "insufficient funds" {
		t.Fatalf("expected insufficient funds, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestTransfer_Success_UpdatesBothAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db)

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
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE accounts SET balance = balance + $1 WHERE user_id=$2")).
		WithArgs(int64(100), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = repo.Deposit(42, 100)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "account not found" {
		t.Fatalf("expected account not found, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}
