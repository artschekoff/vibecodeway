package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"github.com/spf13/cobra"
)

// userEventTypes are the event types a human (or an agent acting for one) may
// write. Source-derived types like comment_added are produced by sync, not here.
var userEventTypes = []string{
	appstore.EventTypeUserNote,
	appstore.EventTypeDecisionMade,
	appstore.EventTypeBlockerIdentified,
	appstore.EventTypeBlockerResolved,
	appstore.EventTypeWorkCompleted,
	appstore.EventTypeStatusUpdate,
	appstore.EventTypeNextStepRecommendation,
	appstore.EventTypeOpenQuestionRaised,
	appstore.EventTypeOpenQuestionAnswered,
	appstore.EventTypeRequirementStated,
	appstore.EventTypeRequirementConfirmed,
	appstore.EventTypeRequirementRejected,
	appstore.EventTypeCurrentStepOverride,
}

func validateUserEventType(t string) error {
	for _, allowed := range userEventTypes {
		if t == allowed {
			return nil
		}
	}
	return fmt.Errorf("--type must be one of: %s", strings.Join(userEventTypes, ", "))
}

// newEventAddCmd appends a user-authored event to a context's event log.
func newEventAddCmd() *cobra.Command {
	var contextID, contextKey, summary, eventType, reference string

	cmd := &cobra.Command{
		Use:   "event-add",
		Short: "Add a user-authored event to a context's event log",
		RunE: func(_ *cobra.Command, _ []string) error {
			if summary == "" {
				return fmt.Errorf("--summary is required")
			}
			if err := validateUserEventType(eventType); err != nil {
				return err
			}

			ctx := context.Background()
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck

			id, err := resolveContextID(ctx, env, contextID, contextKey)
			if err != nil {
				return err
			}

			evt, err := env.store.AddEvent(ctx, appstore.Event{
				ContextID:    id,
				Type:         eventType,
				Timestamp:    time.Now().UTC().Format(time.RFC3339),
				SourceType:   appstore.EventSourceUser,
				Summary:      summary,
				RawReference: reference,
			})
			if err != nil {
				return fmt.Errorf("adding event: %w", err)
			}
			// Mirrors App.AddUserEvent: a new event invalidates the AI snapshot.
			if err := env.store.MarkSnapshotStale(ctx, id); err != nil {
				env.logger.Sugar().Warnf("event-add: marking snapshot stale failed: %v", err)
			}
			return writeJSON(evt)
		},
	}

	cmd.Flags().StringVar(&contextID, "context-id", "", "Context UUID")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Context key, e.g. 'sprint-42'")
	cmd.Flags().StringVar(&summary, "summary", "", "One or two sentences describing what happened")
	cmd.Flags().StringVar(&eventType, "type", appstore.EventTypeUserNote, "Event type")
	cmd.Flags().StringVar(&reference, "reference", "", "Original text or URL this event came from")
	_ = cmd.MarkFlagRequired("summary")

	return setAgentDocs(
		cmd,
		"Add a user-authored event to a context's event log and mark the AI snapshot stale. "+
			"Events appear in the Activity Timeline. Use for notes, decisions, blockers and status updates; "+
			"use add-text instead for long-form material such as a task assignment.",
		"--context-id UUID | --context-key STRING (one required)\n"+
			"--summary STRING (required)\n"+
			"--type STRING (optional; default 'user_note'; one of "+strings.Join(userEventTypes, ", ")+")\n"+
			"--reference STRING (optional; original text or URL)",
		`{"id":"uuid","contextId":"uuid","type":"user_note","timestamp":"RFC3339","sourceType":"user","sourceId":"","author":"","summary":"string","rawReference":"string","createdAt":"RFC3339"}`,
		`vibecodeway-cli event-add --context-key sprint-42 --summary "QA found a regression in login" --type blocker_identified`,
	)
}
