package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/artschekoff/context-consolidator/internal/provider"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	"go.uber.org/zap"
)

// SlackProvider implements provider.Provider for Slack.
type SlackProvider struct {
	slackRepo *slackstore.Repo
	slack     SlackRunner
	logger    *zap.Logger
}

// NewSlackProvider constructs a SlackProvider.
func NewSlackProvider(slackRepo *slackstore.Repo, slack SlackRunner, logger *zap.Logger) *SlackProvider {
	return &SlackProvider{slackRepo: slackRepo, slack: slack, logger: logger}
}

// Name implements provider.Provider.
func (p *SlackProvider) Name() string { return "slack" }

// IsConfigured implements provider.Provider.
func (p *SlackProvider) IsConfigured(ctx context.Context, contextID string) (bool, error) {
	channels, err := p.slackRepo.ListSlackChannels(ctx, contextID, false)
	if err != nil {
		return false, err
	}
	if len(channels) > 0 {
		return true, nil
	}
	threads, err := p.slackRepo.ListSlackThreads(ctx, contextID, false)
	if err != nil {
		return false, err
	}
	if len(threads) > 0 {
		return true, nil
	}
	dmConfigs, err := p.slackRepo.ListSlackDMConfigs(ctx, contextID, false)
	if err != nil {
		return false, err
	}
	return len(dmConfigs) > 0, nil
}

// Refresh reloads all Slack threads stored for contextID.
// Stats.ItemsRefreshed = threads reloaded.
func (p *SlackProvider) Refresh(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.reload(ctx, contextID)
}

// Reload is equivalent to Refresh for Slack (no JQL-style discovery; channels are configured manually).
func (p *SlackProvider) Reload(ctx context.Context, contextID string) (provider.Stats, error) {
	return p.reload(ctx, contextID)
}

func (p *SlackProvider) reload(ctx context.Context, contextID string) (provider.Stats, error) {
	threads, err := p.slackRepo.ListSlackThreads(ctx, contextID, false)
	if err != nil {
		return provider.Stats{}, fmt.Errorf("listing threads: %w", err)
	}

	stats := provider.Stats{}

	// Refresh DM configs.
	dmConfigs, err := p.slackRepo.ListSlackDMConfigs(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("slack refresh: listing dm configs failed", zap.String("contextID", contextID), zap.Error(err))
	}
	for _, cfg := range dmConfigs {
		flags := []string{cfg.Workspace, "--with-messages"}
		if cfg.StartFrom != "" {
			flags = append(flags, "--start-from", cfg.StartFrom)
		}
		raw, err := p.slack.Run(ctx, []string{"list-dms"}, flags)
		if err != nil {
			p.logger.Warn("slack refresh: list-dms failed",
				zap.String("contextID", contextID), zap.String("workspace", cfg.Workspace), zap.Error(err))
			continue
		}
		content := formatDMContent(raw)
		if content == "" {
			continue
		}
		if err := p.slackRepo.UpdateSlackDMConfigContent(ctx, cfg.ID, content); err != nil {
			p.logger.Warn("slack refresh: updating dm config content failed",
				zap.String("contextID", contextID), zap.Error(err))
			continue
		}
		stats.NewItems++
	}

	// Build channelID → SlackChannel lookup so we can forward start_from per channel.
	channelList, err := p.slackRepo.ListSlackChannels(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("slack refresh: listing channels for start_from lookup failed",
			zap.String("contextID", contextID), zap.Error(err))
	}
	channelByID := make(map[string]slackstore.SlackChannel, len(channelList))
	for _, ch := range channelList {
		channelByID[ch.ChannelID] = ch
	}

	// Re-fetch content for every thread already in the database.
	for _, t := range threads {
		var (
			content  string
			fetchErr error
		)

		chStartFrom := channelByID[t.ChannelID].StartFrom

		if t.ThreadTS == "" {
			// This record was created by search-channels (whole-channel snapshot).
			// Re-fetch by calling search-channels again with the channel name.
			if t.ChannelName == "" {
				p.logger.Warn("slack refresh: skipping channel snapshot with empty channel name",
					zap.String("contextID", contextID), zap.String("channelID", t.ChannelID))
				continue
			}
			flags := []string{t.Workspace, t.ChannelName}
			if chStartFrom != "" {
				flags = append(flags, "--start-from", chStartFrom)
			}
			content, fetchErr = p.slack.Run(ctx, []string{"search-channels"}, flags)
			if fetchErr != nil {
				p.logger.Warn("slack refresh: search-channels failed",
					zap.String("contextID", contextID), zap.String("channel", t.ChannelName), zap.Error(fetchErr))
				continue
			}
			// search-channels returns a JSON array; extract the content field of the matching entry.
			content = extractChannelContent(content, t.ChannelID)
		} else {
			flags := []string{t.Workspace, t.ChannelID, t.ThreadTS}
			if t.ChannelName != "" {
				flags = append(flags, "--channel-name", t.ChannelName)
			}
			if t.Permalink != "" {
				flags = append(flags, "--permalink", t.Permalink)
			}
			if t.SearchQuery != "" {
				flags = append(flags, "--search-query", t.SearchQuery)
			}
			if chStartFrom != "" {
				flags = append(flags, "--start-from", chStartFrom)
			}
			content, fetchErr = p.slack.Run(ctx, []string{"load-context"}, flags)
			if fetchErr != nil {
				p.logger.Warn("slack refresh: load context failed",
					zap.String("contextID", contextID), zap.String("channel", t.ChannelID), zap.Error(fetchErr))
				continue
			}
		}

		if content == "" {
			continue
		}

		_, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
			Workspace:   t.Workspace,
			ChannelID:   t.ChannelID,
			ThreadTS:    t.ThreadTS,
			ChannelName: t.ChannelName,
			Permalink:   t.Permalink,
			SearchQuery: t.SearchQuery,
			Content:     content,
		})
		if upsertErr != nil {
			p.logger.Warn("slack refresh: upsert thread failed",
				zap.String("contextID", contextID), zap.Error(upsertErr))
			continue
		}
		stats.ItemsRefreshed++
	}

	// Discover threads for configured channels that have no threads yet.
	// This populates the initial content when a channel is first added.
	channels, err := p.slackRepo.ListSlackChannels(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("slack refresh: list channels failed", zap.String("contextID", contextID), zap.Error(err))
		return stats, nil
	}

	// Build a set of channel IDs that already have at least one thread.
	covered := make(map[string]bool, len(threads))
	for _, t := range threads {
		covered[t.ChannelID] = true
	}

	for _, ch := range channels {
		if covered[ch.ChannelID] {
			continue
		}
		p.discoverAndLoadThreads(ctx, contextID, ch, &stats)
	}

	return stats, nil
}

// discoverAndLoadThreads fetches full channel content for ch (which has no threads yet)
// by calling search-channels — the same command used when the channel is first added.
func (p *SlackProvider) discoverAndLoadThreads(ctx context.Context, contextID string, ch slackstore.SlackChannel, stats *provider.Stats) {
	if ch.ChannelName == "" {
		p.logger.Warn("slack discover: skipping channel with empty name",
			zap.String("channelID", ch.ChannelID))
		return
	}
	flags := []string{ch.Workspace, ch.ChannelName}
	if ch.StartFrom != "" {
		flags = append(flags, "--start-from", ch.StartFrom)
	}
	raw, err := p.slack.Run(ctx, []string{"search-channels"}, flags)
	if err != nil {
		p.logger.Warn("slack discover: search-channels failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
		return
	}

	p.logger.Debug("slack discover: search-channels raw output",
		zap.String("channel", ch.ChannelName),
		zap.String("channelID", ch.ChannelID),
		zap.Int("raw_bytes", len(raw)))

	content := extractChannelContent(raw, ch.ChannelID)
	if content == "" {
		p.logger.Warn("slack discover: no content extracted from search-channels output",
			zap.String("channel", ch.ChannelName),
			zap.String("channelID", ch.ChannelID))
		return
	}

	if _, err = p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
		Workspace:   ch.Workspace,
		ChannelID:   ch.ChannelID,
		ThreadTS:    "",
		ChannelName: ch.ChannelName,
		Content:     content,
	}); err != nil {
		p.logger.Warn("slack discover: upsert thread failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
		return
	}
	stats.ItemsRefreshed++
}

// extractChannelContent parses the JSON array returned by search-channels, finds the
// channel matching channelID, and formats its messages into a human-readable string
// suitable for storage as a slack_thread content.
func extractChannelContent(raw, channelID string) string {
	if raw == "" {
		return ""
	}
	var results []SlackChannelResult
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return ""
	}

	var ch *SlackChannelResult
	for i := range results {
		if results[i].ID == channelID {
			ch = &results[i]
			break
		}
	}
	if ch == nil || len(ch.Messages) == 0 {
		return ""
	}

	return formatMessages(ch.Name, ch.Messages)
}

// formatMessages renders a slice of slackMessages into a readable plain-text block.
func formatMessages(channelName string, msgs []slackMessage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# #%s\n\n", channelName)
	for _, m := range msgs {
		fmt.Fprintf(&sb, "[%s] %s: %s\n", m.Timestamp, m.displayUser(), m.Text)
	}
	return sb.String()
}

// WriteContext appends active Slack channels, threads, and DM configs for contextID to w.
func (p *SlackProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, startAt time.Time) error {
	channels, err := p.slackRepo.ListSlackChannels(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading channels: %w", err)
	}
	threads, err := p.slackRepo.ListSlackThreads(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading threads: %w", err)
	}
	dmConfigs, err := p.slackRepo.ListSlackDMConfigs(ctx, contextID, true)
	if err != nil {
		return fmt.Errorf("loading dm configs: %w", err)
	}
	writeSlackSection(w, channels, threads, dmConfigs, startAt)
	return nil
}

// writeSlackSection renders Slack channels, threads and DM configs as human-readable text into w.
// Threads older than startAt are skipped when startAt is non-zero.
func writeSlackSection(w io.Writer, channels []slackstore.SlackChannel, threads []slackstore.SlackThread, dmConfigs []slackstore.SlackDMConfig, startAt time.Time) {
	filtered := make([]slackstore.SlackThread, 0, len(threads))
	for _, t := range threads {
		if !startAt.IsZero() {
			// Treat unparseable timestamps as "include" to avoid silent data loss.
			ct := ParseTime(t.CreatedAt)
			if !ct.IsZero() && ct.Before(startAt) {
				continue
			}
		}
		filtered = append(filtered, t)
	}

	_, _ = fmt.Fprintf(w, "--- SLACK (%d channel%s, %d thread%s, %d DM config%s) ---\n\n",
		len(channels), plural(len(channels)),
		len(filtered), plural(len(filtered)),
		len(dmConfigs), plural(len(dmConfigs)),
	)

	if len(channels) > 0 {
		_, _ = fmt.Fprint(w, "Monitored channels: ")
		names := make([]string, len(channels))
		for i, ch := range channels {
			names[i] = fmt.Sprintf("#%s (%s)", ch.ChannelName, ch.Workspace)
		}
		_, _ = fmt.Fprint(w, strings.Join(names, ", "))
		_, _ = fmt.Fprint(w, "\n\n")
	}

	if len(filtered) == 0 && len(dmConfigs) == 0 {
		_, _ = fmt.Fprint(w, "No Slack threads match the given criteria.\n\n")
		return
	}

	for _, t := range filtered {
		chanLabel := t.ChannelName
		if chanLabel == "" {
			chanLabel = t.ChannelID
		}
		_, _ = fmt.Fprintf(w, "#%s (%s)", chanLabel, t.Workspace)
		if t.Permalink != "" {
			_, _ = fmt.Fprintf(w, " — %s", t.Permalink)
		}
		_, _ = fmt.Fprint(w, "\n")

		if t.Content != "" {
			_, _ = fmt.Fprint(w, t.Content)
		} else {
			_, _ = fmt.Fprint(w, "(no content — run Refresh to load)")
		}
		_, _ = fmt.Fprint(w, "\n\n")
	}

	for _, dm := range dmConfigs {
		label := fmt.Sprintf("DMs (%s)", dm.Workspace)
		if dm.StartFrom != "" {
			label += fmt.Sprintf(" since %s", dm.StartFrom)
		}
		_, _ = fmt.Fprintf(w, "%s\n", label)
		if dm.Content != "" {
			_, _ = fmt.Fprint(w, dm.Content)
		} else {
			_, _ = fmt.Fprint(w, "(no content — run Refresh to load)")
		}
		_, _ = fmt.Fprint(w, "\n\n")
	}
}
