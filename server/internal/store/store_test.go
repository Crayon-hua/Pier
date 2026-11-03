package store

import (
	"errors"
	"testing"
)

func TestTransactionRollbackAndRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	expected := errors.New("reject")
	if e = s.Update(func(d *Data) error { d.Projects["bad"] = Project{ID: "bad"}; return expected }); !errors.Is(e, expected) {
		t.Fatal(e)
	}
	if len(s.Snapshot().Projects) != 0 {
		t.Fatal("rejected transaction changed live state")
	}
	if e = s.Update(func(d *Data) error { d.Tasks["t"] = Task{ID: "t", Status: "running"}; return nil }); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	task := reopened.Snapshot().Tasks["t"]
	if task.Status != "interrupted" || task.FinishedAt == nil {
		t.Fatal("restart guessed interrupted task succeeded")
	}
}
