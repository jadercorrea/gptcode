package ledger

import (
	"errors"
	"testing"
)

func TestTransfer(t *testing.T) {
	from := NewAccount(100)
	to := NewAccount(25)

	if err := Transfer(from, to, 40); err != nil {
		t.Fatalf("Transfer() error = %v", err)
	}
	if got := from.Balance(); got != 60 {
		t.Errorf("source balance = %d, want 60", got)
	}
	if got := to.Balance(); got != 65 {
		t.Errorf("destination balance = %d, want 65", got)
	}
}

func TestTransferRejectsInsufficientFunds(t *testing.T) {
	from := NewAccount(10)
	to := NewAccount(20)

	if err := Transfer(from, to, 11); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("Transfer() error = %v, want ErrInsufficientFunds", err)
	}
	if got := from.Balance() + to.Balance(); got != 30 {
		t.Errorf("total balance = %d, want 30", got)
	}
}
