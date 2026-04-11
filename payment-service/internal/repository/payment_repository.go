package repository

import (
	"database/sql"
	"errors"
)

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) CreateAccount(userID int64) error {
	_, err := r.db.Exec(
		"INSERT INTO accounts (user_id, balance) VALUES ($1, 0)",
		userID,
	)
	return err
}

func (r *PaymentRepository) GetBalance(userID int64) (int64, error) {
	var balance int64
	err := r.db.QueryRow(
		"SELECT balance FROM accounts WHERE user_id=$1",
		userID,
	).Scan(&balance)

	return balance, err
}

func (r *PaymentRepository) Transfer(fromID, toID int64, amount int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var balance int64
	err = tx.QueryRow(
		"SELECT balance FROM accounts WHERE user_id=$1 FOR UPDATE",
		fromID,
	).Scan(&balance)

	if err != nil {
		return err
	}

	if balance < amount {
		return errors.New("insufficient funds")
	}

	res, err := tx.Exec(
		"UPDATE accounts SET balance = balance - $1 WHERE user_id=$2",
		amount, fromID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("debit failed: sender account missing")
	}

	res, err = tx.Exec(
		"UPDATE accounts SET balance = balance + $1 WHERE user_id=$2",
		amount, toID,
	)
	if err != nil {
		return err
	}
	n, err = res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("recipient account not found")
	}

	return tx.Commit()
}

func (r *PaymentRepository) Deposit(userID int64, amount int64) error {
	_, err := r.db.Exec(
		"UPDATE accounts SET balance = balance + $1 WHERE user_id=$2",
		amount, userID,
	)
	return err
}
