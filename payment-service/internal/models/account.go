package models

// Account balances are stored in minor currency units (e.g. kopecks).
type Account struct {
	UserID  int64
	Balance int64
}
