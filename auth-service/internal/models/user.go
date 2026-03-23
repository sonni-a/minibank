package models

type AuthUser struct {
	ID           int64
	Email        string
	PasswordHash string
}
