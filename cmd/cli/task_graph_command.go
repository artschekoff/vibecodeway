package main

import (
	"context"
	"fmt"
	"time"

	"github.com/artschekoff/vibecodeway/internal/taskgraph"
	"github.com/artschekoff/vibecodeway/internal/tracker"
	"github.com/artschekoff/vibecodeway/internal/tracker/jiratracker"
	"github.com/spf13/cobra"
)

// task-graph: dump the board's task graph (my issues, epics, blockers,
// ready/waiting) for debugging and for Claude.
func newTaskGraphCmd() *cobra.Command {
	var provider string
	var refresh, force bool
	cmd := &cobra.Command{
		Use:   "task-graph",
		Short: "Print the task graph (my issues, epics, blockers, ready/waiting)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env, err := openServiceEnv()
			if err != nil {
				return err
			}
			defer env.store.Close() //nolint:errcheck
			tracker.Register(jiratracker.New(env.svc))
			p, err := tracker.Get(provider)
			if err != nil {
				return err
			}
			ctx := context.Background()
			if refresh {
				r := &taskgraph.Refresher{Store: env.store, Workers: 4, MineTTL: time.Hour, OtherTTL: 24 * time.Hour,
					Logf: func(f string, a ...any) { fmt.Fprintf(cmd.ErrOrStderr(), f+"\n", a...) }}
				if err := r.Refresh(ctx, p, force); err != nil {
					return err
				}
			}
			rows, edgeRows, err := env.store.ListTaskGraph(ctx, provider)
			if err != nil {
				return err
			}
			nodes, edges := taskgraph.FromStore(rows, edgeRows, p.IsEpic)
			return writeJSON(taskgraph.Build(provider, nodes, edges))
		},
	}
	cmd.Flags().StringVar(&provider, "provider", jiratracker.Name, "tracker provider (jira)")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "pull fresh data from the tracker before printing")
	cmd.Flags().BoolVar(&force, "force", false, "with --refresh: ignore detail caches (one tracker call per issue)")
	return setAgentDocs(cmd,
		"Dump the cached task graph: nodes (my issues + epics/blockers), edges (parent/blocks/relates), ready and waiting keys.",
		"--provider jira [--refresh] [--force]",
		`{"provider":"jira","nodes":[{"key":"JH-1","summary":"…","status":"To Do","type":"Subtask","priority":"High","priorityRank":4,"mine":true,"blocked":true,"blockedBy":["JH-2"],"epic":false}],"edges":[{"src":"JH-1","dst":"JH-10","kind":"parent"}],"ready":["JH-3"],"waiting":["JH-1"]}`,
		`vibecodeway-cli task-graph --refresh`)
}
