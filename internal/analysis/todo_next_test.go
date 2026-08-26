package analysis

import (
	"reflect"
	"strings"
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

func TestComputeImplementTickets(t *testing.T) {
	issues := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Status: "To Do"},          // actionable
		{Key: "JH-2", Status: "In Progress"},    // actionable
		{Key: "JH-3", Status: "Done"},           // terminal → skip
		{Key: "JH-4", Status: "Ready for QA"},   // downstream/waiting → skip
		{Key: "JH-5", Status: "Ready to Merge"}, // downstream → skip
		{Key: "JH-6", Status: "Closed"},         // terminal → skip
		{Key: "JH-7", Status: "In Development"}, // actionable
		{Key: "JH-8", Status: "  to do  "},      // normalized (trim+case) → actionable
	}
	got := computeImplementTickets(issues)
	want := []string{"JH-1", "JH-2", "JH-7", "JH-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("computeImplementTickets = %v, want %v", got, want)
	}
}

// activeTickets drops fully-terminal tickets but keeps downstream/waiting ones
// (which still warrant check_status) so the board the AI sees isn't buried.
func TestActiveTicketsDropsTerminalKeepsWaiting(t *testing.T) {
	issues := []appstore.BoardJiraAssignment{
		{Key: "JH-1", Status: "To Do"},
		{Key: "JH-2", Status: "Done"},         // dropped
		{Key: "JH-3", Status: "Ready for QA"}, // kept (check_status)
		{Key: "JH-4", Status: "Closed"},       // dropped
	}
	got := activeTickets(issues)
	var keys []string
	for _, it := range got {
		keys = append(keys, it.Key)
	}
	want := []string{"JH-1", "JH-3"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("activeTickets keys = %v, want %v", keys, want)
	}
}

func TestTodoNextPromptSurfacesImplementTicketsAndDeclutters(t *testing.T) {
	in := TodoInput{
		JiraIssues: []appstore.BoardJiraAssignment{
			{Key: "JH-1", Summary: "Build thing", Status: "To Do"},
			{Key: "JH-9", Summary: "Old done work", Status: "Done"},
		},
		ImplementTicketKeys: []string{"JH-1"},
		StalePRNumbers:      []int{42},
	}
	got := todoNextPrompt(defaultTodoNextPrompt, in)

	if !strings.Contains(got, "IMPLEMENT_TICKETS (assigned tickets that are open work to build): [JH-1]") {
		t.Fatalf("prompt must surface the implement tickets line:\n%s", got)
	}
	if !strings.Contains(got, "JH-1") {
		t.Fatalf("prompt must include actionable ticket JH-1:\n%s", got)
	}
	if strings.Contains(got, "JH-9") {
		t.Fatalf("terminal (Done) ticket JH-9 must be decluttered from the board:\n%s", got)
	}
}
