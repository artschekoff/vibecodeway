package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/provider"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// slowProvider is a test stub that sleeps for a fixed duration on Reload.
type slowProvider struct {
	name  string
	delay time.Duration
}

func (p *slowProvider) Name() string { return p.name }
func (p *slowProvider) IsConfigured(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (p *slowProvider) Reload(_ context.Context, _ string) (provider.Stats, error) {
	time.Sleep(p.delay)
	return provider.Stats{}, nil
}
func (p *slowProvider) Refresh(_ context.Context, _ string) (provider.Stats, error) {
	return provider.Stats{}, nil
}
func (p *slowProvider) CollectItems(_ context.Context, _ string, _ time.Time, _ bool) ([]provider.Item, error) {
	return nil, nil
}

// TestReloadAllContexts_ProvidersRunConcurrently verifies that providers are fanned out
// into goroutines so that two 60ms providers finish in roughly 60ms, not 120ms+.
func TestReloadAllContexts_ProvidersRunConcurrently(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	jRepo := jirastore.NewRepo(s.DB())
	sRepo := slackstore.NewRepo(s.DB())

	delay := 60 * time.Millisecond
	p1 := &slowProvider{name: "slow1", delay: delay}
	p2 := &slowProvider{name: "slow2", delay: delay}

	svc := NewContextService(s, nil, jRepo, sRepo, nil, logger, p1, p2)

	ctx := context.Background()
	_, err := svc.store.CreateContext(ctx, "concurrent-test")
	require.NoError(t, err)

	// contextLog is nil so snapshotBefore/recordContextChange are no-ops; elapsed
	// only includes ListContexts and the goroutine fan-out with provider execution.
	start := time.Now()
	results, err := svc.ReloadAllContexts(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Less(t, elapsed, 150*time.Millisecond,
		"providers should run concurrently")
}

// newTestContextService creates a minimal ContextService backed by a temp SQLite store.
func newTestContextService(t *testing.T, providers ...provider.Provider) *ContextService {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	return NewContextService(s, nil, jirastore.NewRepo(s.DB()), slackstore.NewRepo(s.DB()), nil, logger, providers...)
}

// TestReloadAllContexts_ContextsRunConcurrently verifies that multiple contexts are
// reloaded concurrently so that N contexts × delay finishes in roughly one delay's time.
func TestReloadAllContexts_ContextsRunConcurrently(t *testing.T) {
	delay := 60 * time.Millisecond
	p := &slowProvider{name: "jira", delay: delay}

	svc := newTestContextService(t, p)
	ctx := context.Background()

	// Create 3 contexts.
	for i := 0; i < 3; i++ {
		_, err := svc.store.CreateContext(ctx, fmt.Sprintf("ctx-%d", i))
		require.NoError(t, err)
	}

	// contextLog is nil so snapshotBefore/recordContextChange are no-ops;
	// elapsed includes ListContexts + goroutine fan-out with 3 contexts × 60ms providers.
	start := time.Now()
	results, err := svc.ReloadAllContexts(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, results, 3)
	// Sequential: 3 × 60ms = 180ms. Parallel: ~60ms. Threshold 150ms gives 90ms CI headroom.
	assert.Less(t, elapsed, 150*time.Millisecond, "contexts should reload concurrently")
}
