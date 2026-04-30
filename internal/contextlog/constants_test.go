package contextlog_test

import (
	"testing"

	"github.com/artschekoff/context-consolidator/internal/contextlog"
)

func TestTriggerAgent_value(t *testing.T) {
	if contextlog.TriggerAgent != "agent" {
		t.Fatalf("expected TriggerAgent == %q, got %q", "agent", contextlog.TriggerAgent)
	}
}
