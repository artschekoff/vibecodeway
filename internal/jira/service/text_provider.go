package service

import (
	"context"
	"fmt"
	"strings"
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

// CollectItems returns one Item per text entry and uploaded file for global sorting.
// startAt is ignored because texts are not time-stamped data.
func (p *TextProvider) CollectItems(ctx context.Context, contextID string, _ time.Time, _ bool) ([]provider.Item, error) {
	texts, err := p.store.ListContextTexts(ctx, contextID, true)
	if err != nil {
		return nil, fmt.Errorf("loading context texts: %w", err)
	}
	uploaded, err := p.store.ListUploadedFiles(ctx, contextID, true)
	if err != nil {
		return nil, fmt.Errorf("loading uploaded files: %w", err)
	}

	var items []provider.Item

	for _, t := range texts {
		ts := ParseTime(t.CreatedAt)
		name := t.Name
		if name == "" {
			name = "(untitled)"
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · TEXT] %s\n\n", FormatDateTime(t.CreatedAt), name)
		fmt.Fprint(&buf, t.Content)
		fmt.Fprint(&buf, "\n\n")
		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}

	for _, f := range uploaded {
		ts := ParseTime(f.CreatedAt)
		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · FILE] %s (%s)\n\n", FormatDateTime(f.CreatedAt), f.OriginalName, f.FileType)
		fmt.Fprint(&buf, f.ExtractedText)
		fmt.Fprint(&buf, "\n\n")
		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}
	return items, nil
}
