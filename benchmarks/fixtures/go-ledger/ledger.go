package ledger

import (
	"errors"
	"sync"
)

var ErrInsufficientFunds = errors.New("insufficient funds")

type Account struct {
	mu      sync.Mutex
	balance int
}

func NewAccount(balance int) *Account {
	return &Account{balance: balance}
}

func (a *Account) Balance() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.balance
}

// Transfer moves funds while preserving the total held by both accounts.
func Transfer(from, to *Account, amount int) error {
	from.mu.Lock()
	defer from.mu.Unlock()
	to.mu.Lock()
	defer to.mu.Unlock()

	if from.balance < amount {
		return ErrInsufficientFunds
	}
	from.balance -= amount
	to.balance += amount
	return nil
}
