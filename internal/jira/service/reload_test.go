package service

import (
	"context"
	"io"
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
func (p *slowProvider) WriteContext(_ context.Context, _ io.Writer, _ string, _ time.Time, _ bool) error {
	return nil
}

// TestReloadAllContexts_ProvidersRunConcurrently verifies that providers are fanned out
// into goroutines so that two 50ms providers finish in roughly 50ms, not 100ms+.
func TestReloadAllContexts_ProvidersRunConcurrently(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	s := openTestStore(t)
	jRepo := jirastore.NewRepo(s.DB())
	sRepo := slackstore.NewRepo(s.DB())

	p1 := &slowProvider{name: "slow1", delay: 50 * time.Millisecond}
	p2 := &slowProvider{name: "slow2", delay: 50 * time.Millisecond}

	svc := NewContextService(s, nil, jRepo, sRepo, nil, logger, p1, p2)

	ctx := context.Background()
	_, err := svc.store.CreateContext(ctx, "concurrent-test")
	require.NoError(t, err)

	start := time.Now()
	results, err := svc.ReloadAllContexts(ctx)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.Less(t, elapsed, 80*time.Millisecond,
		"two 50ms providers in parallel should complete in <80ms, got %v", elapsed)
}
