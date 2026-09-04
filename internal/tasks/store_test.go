package tasks

import "testing"

func TestStoreKeepsNewestTasks(t *testing.T) {
	store := NewStore(2)

	store.Add(Task{ID: "1", Printer: "A", Status: StatusReceived})
	store.Add(Task{ID: "2", Printer: "A", Status: StatusSubmitted})
	store.Add(Task{ID: "3", Printer: "A", Status: StatusFailed, Error: "offline"})

	got := store.Recent()
	if len(got) != 2 {
		t.Fatalf("Recent length = %d, want 2", len(got))
	}
	if got[0].ID != "3" || got[1].ID != "2" {
		t.Fatalf("Recent IDs = %q,%q; want 3,2", got[0].ID, got[1].ID)
	}
}

func TestStoreUpdatesExistingTask(t *testing.T) {
	store := NewStore(5)
	store.Add(Task{ID: "1", Printer: "A", Status: StatusReceived})

	store.Update("1", StatusSubmitted, "")

	got := store.Recent()
	if got[0].Status != StatusSubmitted {
		t.Fatalf("Status = %q, want %q", got[0].Status, StatusSubmitted)
	}
}
