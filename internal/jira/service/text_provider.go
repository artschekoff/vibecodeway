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

// IsConfigured reports whether the context has any texts or uploaded documents attached.
func (p *TextProvider) IsConfigured(ctx context.Context, contextID string) (bool, error) {
	texts, err := p.store.ListContextTexts(ctx, contextID, false)
	if err != nil {
		return false, fmt.Errorf("checking context texts: %w", err)
	}
	if len(texts) > 0 {
		return true, nil
	}
	uploaded, err := p.store.ListUploadedFiles(ctx, contextID, false)
	if err != nil {
		return false, fmt.Errorf("checking uploaded files: %w", err)
	}
	return len(uploaded) > 0, nil
}

// Refresh is a no-op for text files — they are static content managed by the user.
func (p *TextProvider) Refresh(_ context.Context, _ string) (provider.Stats, error) {
	return provider.Stats{}, nil
}

// Reload is a no-op for text files — they are static content managed by the user.
func (p *TextProvider) Reload(_ context.Context, _ string) (provider.Stats, error) {
	return provider.Stats{}, nil
}

// WriteContext appends active texts and uploaded documents for the context to w.
// startAt is ignored because texts are not time-stamped data.
func (p *TextProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, _ time.Time) error {
	texts, err := p.store.ListContextTexts(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading context texts: %w", err)
	}

	if len(texts) > 0 {
		_, _ = fmt.Fprintf(w, "--- TEXTS (%d text%s) ---\n\n", len(texts), plural(len(texts)))
		for _, t := range texts {
			if t.Name != "" {
				_, _ = fmt.Fprintf(w, "### %s\n\n", t.Name)
			}
			_, _ = fmt.Fprint(w, t.Content)
			_, _ = fmt.Fprint(w, "\n\n")
		}
	}

	uploaded, err := p.store.ListUploadedFiles(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading uploaded files: %w", err)
	}

	if len(uploaded) > 0 {
		_, _ = fmt.Fprintf(w, "--- UPLOADED FILES (%d file%s) ---\n\n", len(uploaded), plural(len(uploaded)))
		for _, f := range uploaded {
			_, _ = fmt.Fprintf(w, "### %s (%s)\n\n", f.OriginalName, f.FileType)
			_, _ = fmt.Fprint(w, f.ExtractedText)
			_, _ = fmt.Fprint(w, "\n\n")
		}
	}

	return nil
}
