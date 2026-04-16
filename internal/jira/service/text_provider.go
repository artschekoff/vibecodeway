package service

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/artschekoff/context-consolidator/internal/provider"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	"go.uber.org/zap"
)

const providerText = "text"

// TextProvider implements provider.Provider for plain-text documents attached to a context.
// Text files are static — Refresh and Reload are no-ops since they are managed manually.
type TextProvider struct {
	store  *appstore.Store
	logger *zap.Logger
}

// NewTextProvider constructs a TextProvider.
func NewTextProvider(store *appstore.Store, logger *zap.Logger) *TextProvider {
	return &TextProvider{store: store, logger: logger}
}

// Name implements provider.Provider.
func (p *TextProvider) Name() string { return providerText }

// IsConfigured reports whether the context has any text files attached.
func (p *TextProvider) IsConfigured(ctx context.Context, contextID string) (bool, error) {
	files, err := p.store.ListContextFiles(ctx, contextID, false)
	if err != nil {
		return false, fmt.Errorf("checking text files: %w", err)
	}
	return len(files) > 0, nil
}

// Refresh is a no-op for text files — they are static content managed by the user.
func (p *TextProvider) Refresh(_ context.Context, _ string) (provider.Stats, error) {
	return provider.Stats{}, nil
}

// Reload is a no-op for text files — they are static content managed by the user.
func (p *TextProvider) Reload(_ context.Context, _ string) (provider.Stats, error) {
	return provider.Stats{}, nil
}

// WriteContext appends active text files for the context to w.
// startAt is ignored because text files are not time-stamped data.
func (p *TextProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, _ time.Time) error {
	files, err := p.store.ListContextFiles(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading text files: %w", err)
	}

	if len(files) == 0 {
		return nil
	}

	_, _ = fmt.Fprintf(w, "--- TEXT FILES (%d file%s) ---\n\n", len(files), plural(len(files)))
	for _, f := range files {
		if f.Name != "" {
			_, _ = fmt.Fprintf(w, "### %s\n\n", f.Name)
		}
		_, _ = fmt.Fprint(w, f.Content)
		_, _ = fmt.Fprint(w, "\n\n")
	}
	return nil
}
