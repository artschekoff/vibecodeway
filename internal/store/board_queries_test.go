package store

import (
	"context"
	"testing"
)

func TestListBoardBlockers_empty(t *testing.T) {
	db := openTestStore(t)
	blockers, err := db.ListBoardBlockers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blockers) != 0 {
		t.Fatalf("expected 0 blockers, got %d", len(blockers))
	}
}

func TestListBoardAssignments_empty(t *testing.T) {
	db := openTestStore(t)
	assignments, err := db.ListBoardAssignments(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assignments) != 0 {
		t.Fatalf("expected 0 assignments, got %d", len(assignments))
	}
}

func TestListBoardPRsByAuthor_empty(t *testing.T) {
	db := openTestStore(t)
	prs, err := db.ListBoardPRsByAuthor(context.Background(), "alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 0 {
		t.Fatalf("expected 0 PRs, got %d", len(prs))
	}
}
