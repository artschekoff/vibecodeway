package analysis

import (
	"testing"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
)

func TestIsPRStale(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -10).Format(time.RFC3339)
	fresh := now.AddDate(0, 0, -2).Format(time.RFC3339)

	if !isPRStale(old, now, 7) {
		t.Fatal("10 days old with threshold 7 must be stale")
	}
	if isPRStale(fresh, now, 7) {
		t.Fatal("2 days old with threshold 7 must not be stale")
	}
	if isPRStale("", now, 7) {
		t.Fatal("empty updatedAt must not be stale")
	}
	if isPRStale("garbage", now, 7) {
		t.Fatal("unparseable updatedAt must not be stale")
	}
}

func TestComputeStalePRs(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -10).Format(time.RFC3339)
	prs := []appstore.BoardPR{
		{PRNumber: 1, State: "open", ReviewStatus: "review_required", PRUpdatedAt: old},
		{PRNumber: 2, State: "open", ReviewStatus: "approved", PRUpdatedAt: old}, // approved → not actionable
		{PRNumber: 3, State: "open", ReviewStatus: "review_required", PRUpdatedAt: now.Format(time.RFC3339)},
		{PRNumber: 4, State: "closed", ReviewStatus: "review_required", PRUpdatedAt: old}, // not open
	}
	got := computeStalePRs(prs, now, 7)
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("computeStalePRs = %v, want [1]", got)
	}
}
