package ledger

import (
	"sync"
	"testing"
	"time"
)

func TestSelfTransferCompletes(t *testing.T) {
	account := NewAccount(1_000)
	completed := make(chan error, 1)
	go func() {
		completed <- Transfer(account, account, 10)
	}()

	select {
	case err := <-completed:
		if err != nil {
			t.Fatalf("Transfer() error = %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("self-transfer deadlocked")
	}

	if balance := account.Balance(); balance != 1_000 {
		t.Errorf("balance = %d, want 1000", balance)
	}
}

func TestOpposingTransfersComplete(t *testing.T) {
	left := NewAccount(1_000)
	right := NewAccount(1_000)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)

	go func() {
		defer workers.Done()
		<-start
		for range 1_000 {
			_ = Transfer(left, right, 1)
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for range 1_000 {
			_ = Transfer(right, left, 1)
		}
	}()

	close(start)
	completed := make(chan struct{})
	go func() {
		workers.Wait()
		close(completed)
	}()

	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("opposing transfers deadlocked")
	}

	if total := left.Balance() + right.Balance(); total != 2_000 {
		t.Errorf("total balance = %d, want 2000", total)
	}
}
