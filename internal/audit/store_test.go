package audit

import (
	"testing"
	"time"
)

func TestAppendAndQuery(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir())
	now := time.Now().UTC()
	for _, event := range []Event{
		{Actor: "admin", Action: "user.create", Outcome: "success", Target: "alice", Timestamp: now.Add(-time.Minute)},
		{Actor: "admin", Action: "auth.login", Outcome: "failure", Message: "bad password", Timestamp: now},
	} {
		if err := store.Append(event); err != nil {
			t.Fatal(err)
		}
	}
	all, err := store.Query(Filter{Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != 2 || len(all.Items) != 2 {
		t.Fatalf("unexpected result: %+v", all)
	}
	failed, err := store.Query(Filter{Outcome: "failure", Page: 1, Size: 20})
	if err != nil {
		t.Fatal(err)
	}
	if failed.Total != 1 || failed.Items[0].Action != "auth.login" {
		t.Fatalf("unexpected failure filter: %+v", failed)
	}
}
