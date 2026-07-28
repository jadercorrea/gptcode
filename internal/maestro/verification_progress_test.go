package maestro

import "testing"

func TestVerificationProgressStopsAfterThreeEquivalentFailures(t *testing.T) {
	tracker := newVerificationProgress(3)
	outputs := []string{
		"--- FAIL: TestWriteRejectsSymlinkedParent (0.00s)\nFAIL example.com/safestore 0.295s\n",
		"--- FAIL: TestWriteRejectsSymlinkedParent (0.01s)\nFAIL example.com/safestore 0.420s\n",
		"--- FAIL: TestWriteRejectsSymlinkedParent (0.02s)\nFAIL example.com/safestore 0.362s\n",
	}

	for index, output := range outputs {
		plateau := tracker.Observe(output)
		if index < 2 && plateau {
			t.Fatalf("reported plateau after only %d observations", index+1)
		}
		if index == 2 && !plateau {
			t.Fatal("expected three equivalent failures to report a plateau")
		}
	}
}

func TestVerificationProgressResetsWhenFailureChanges(t *testing.T) {
	tracker := newVerificationProgress(3)
	if tracker.Observe("undefined: unsafe") {
		t.Fatal("first failure cannot be a plateau")
	}
	if tracker.Observe("undefined: unsafe") {
		t.Fatal("second failure cannot be a plateau")
	}
	if tracker.Observe("TestSelfTransferCompletes: self-transfer deadlocked") {
		t.Fatal("a changed failure is progress, not a plateau")
	}
	if tracker.Consecutive() != 1 {
		t.Fatalf("consecutive failures = %d, want reset to 1", tracker.Consecutive())
	}
}

func TestVerificationProgressIgnoresGoTemporaryDirectoryIDs(t *testing.T) {
	tracker := newVerificationProgress(3)
	outputs := []string{
		"Write() error = lstat /private/var/folders/aa/one/T/TestWriteStoresNestedFileInsideRoot1722907079/001/reports: no such file\n",
		"Write() error = lstat /private/var/folders/bb/two/T/TestWriteStoresNestedFileInsideRoot3193033618/001/reports: no such file\n",
		"Write() error = lstat /private/var/folders/cc/three/T/TestWriteStoresNestedFileInsideRoot1568810318/001/reports: no such file\n",
	}

	for index, output := range outputs {
		plateau := tracker.Observe(output)
		if index < 2 && plateau {
			t.Fatalf("reported plateau after only %d observations", index+1)
		}
		if index == 2 && !plateau {
			t.Fatal("temporary directory IDs must not disguise an equivalent failure")
		}
	}
}
