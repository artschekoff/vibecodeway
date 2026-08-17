package main

import (
	"strings"
	"testing"
)

func TestValidateUserEventType(t *testing.T) {
	for _, ok := range []string{"user_note", "decision_made", "blocker_identified", "work_completed"} {
		if err := validateUserEventType(ok); err != nil {
			t.Errorf("validateUserEventType(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "note", "USER_NOTE", "jira_status_changed"} {
		if err := validateUserEventType(bad); err == nil {
			t.Errorf("validateUserEventType(%q) must error", bad)
		}
	}
}

func TestValidateUserEventType_ErrorListsAllowedValues(t *testing.T) {
	err := validateUserEventType("nope")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "user_note") {
		t.Fatalf("error must list allowed values, got %q", err)
	}
}
