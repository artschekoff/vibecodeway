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
	stats := provider.Stats{}

	// ── Refresh DM configs ───────────────────────────────────────────────────
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

	// ── Refresh user-added threads (search_query or permalink set) ───────────
	threads, err := p.slackRepo.ListSlackThreads(ctx, contextID, false)
	if err != nil {
		return stats, fmt.Errorf("listing threads: %w", err)
	}

	channels, err := p.slackRepo.ListSlackChannels(ctx, contextID, false)
	if err != nil {
		p.logger.Warn("slack refresh: list channels failed", zap.String("contextID", contextID), zap.Error(err))
	}
	channelByID := make(map[string]slackstore.SlackChannel, len(channels))
	for _, ch := range channels {
		channelByID[ch.ChannelID] = ch
	}

	for _, t := range threads {
		// Channel per-message records (slackChannelID set) are handled by refreshChannelMessages.
		// Old blobs (threadTS="") are cleaned up by refreshChannelMessages.
		if t.SlackChannelID != "" || t.ThreadTS == "" {
			continue
		}
		chStartFrom := channelByID[t.ChannelID].StartFrom
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
		content, fetchErr := p.slack.Run(ctx, []string{"load-context"}, flags)
		if fetchErr != nil {
			p.logger.Warn("slack refresh: load context failed",
				zap.String("contextID", contextID), zap.String("channel", t.ChannelID), zap.Error(fetchErr))
			continue
		}
		if content == "" {
			continue
		}
		if _, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
			Workspace:   t.Workspace,
			ChannelID:   t.ChannelID,
			ThreadTS:    t.ThreadTS,
			ChannelName: t.ChannelName,
			Permalink:   t.Permalink,
			SearchQuery: t.SearchQuery,
			Content:     content,
		}); upsertErr != nil {
			p.logger.Warn("slack refresh: upsert thread failed",
				zap.String("contextID", contextID), zap.Error(upsertErr))
			continue
		}
		stats.ItemsRefreshed++
	}

	// ── Refresh channel messages: one slack_thread record per message ────────
	for _, ch := range channels {
		p.refreshChannelMessages(ctx, contextID, ch, &stats)
	}

	return stats, nil
}

// refreshChannelMessages fetches all messages for ch via search-channels, upserts one
// slack_thread record per top-level message, and deletes any old single-blob records.
func (p *SlackProvider) refreshChannelMessages(ctx context.Context, contextID string, ch slackstore.SlackChannel, stats *provider.Stats) {
	if ch.ChannelName == "" {
		p.logger.Warn("slack refresh: skipping channel with empty name",
			zap.String("channelID", ch.ChannelID))
		return
	}
	flags := []string{ch.Workspace, ch.ChannelName}
	if ch.StartFrom != "" {
		flags = append(flags, "--start-from", ch.StartFrom)
	}
	raw, err := p.slack.Run(ctx, []string{"search-channels"}, flags)
	if err != nil {
		p.logger.Warn("slack refresh: search-channels failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
		return
	}

	msgs := parseChannelMessages(raw, ch.ChannelID)
	if len(msgs) == 0 {
		p.logger.Warn("slack refresh: no messages returned for channel",
			zap.String("channel", ch.ChannelName))
		return
	}

	upserted := 0
	for _, msg := range msgs {
		if msg.RawTS == "" {
			p.logger.Warn("slack refresh: message missing rawTs, skipping",
				zap.String("channel", ch.ChannelName))
			continue
		}
		if _, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
			SlackChannelID: ch.ID,
			Workspace:      ch.Workspace,
			ChannelID:      ch.ChannelID,
			ThreadTS:       msg.RawTS,
			ChannelName:    ch.ChannelName,
			Content:        formatSingleMessage(msg),
		}); upsertErr != nil {
			p.logger.Warn("slack refresh: upsert message thread failed",
				zap.String("channel", ch.ChannelName), zap.String("ts", msg.RawTS), zap.Error(upsertErr))
			continue
		}
		upserted++
		for _, reply := range msg.Replies {
			if reply.RawTS == "" {
				continue
			}
			if _, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
				SlackChannelID: ch.ID,
				Workspace:      ch.Workspace,
				ChannelID:      ch.ChannelID,
				ThreadTS:       reply.RawTS,
				ParentThreadTS: msg.RawTS,
				ChannelName:    ch.ChannelName,
				Content:        formatSingleMessage(reply),
			}); upsertErr != nil {
				p.logger.Warn("slack refresh: upsert reply thread failed",
					zap.String("channel", ch.ChannelName), zap.String("ts", reply.RawTS), zap.Error(upsertErr))
			} else {
				upserted++
			}
		}
	}

	if upserted == 0 {
		return
	}

	// Delete old single-blob records (thread_ts="") now that per-message records exist.
	if err := p.slackRepo.DeleteChannelBlobs(ctx, contextID, ch.ChannelID); err != nil {
		p.logger.Warn("slack refresh: delete channel blobs failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
	}
	stats.ItemsRefreshed += upserted
}

// formatSingleMessage renders one Slack message as plain text (no replies).
// The first line is "[timestamp] user: text"; multi-line text is indented.
// Replies are stored as separate slack_thread records and not included here.
func formatSingleMessage(m slackstore.SlackMessage) string {
	var sb strings.Builder
	lines := strings.Split(m.Text, "\n")
	fmt.Fprintf(&sb, "[%s] %s: %s\n", m.Timestamp, m.DisplayUser(), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(&sb, "  %s\n", line)
	}
	return sb.String()
}

func writeFormattedMessage(sb *strings.Builder, m slackstore.SlackMessage, indent string) {
	lines := strings.Split(m.Text, "\n")
	fmt.Fprintf(sb, "%s[%s] %s: %s\n", indent, m.Timestamp, m.DisplayUser(), lines[0])
	for _, line := range lines[1:] {
		fmt.Fprintf(sb, "%s  %s\n", indent, line)
	}
	for _, r := range m.Replies {
		writeFormattedMessage(sb, r, indent+"  ")
	}
}

// formatMessages renders a full channel's messages as a preview string.
// Used only for SlackChannelResult.Content (search-result preview in ContextForm).
// Storage uses formatSingleMessage per message instead.
func formatMessages(channelName string, msgs []slackstore.SlackMessage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# #%s\n\n", channelName)
	for _, m := range msgs {
		writeFormattedMessage(&sb, m, "")
		sb.WriteString("\n")
	}
	return sb.String()
}

// parseChannelMessages parses the JSON array from search-channels and returns
// the messages for the channel matching channelID, in oldest-first order.
func parseChannelMessages(raw, channelID string) []slackstore.SlackMessage {
	if raw == "" {
		return nil
	}
	var results []SlackChannelResult
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return nil
	}
	for i := range results {
		if results[i].ID == channelID {
			msgs := results[i].Messages
			// Reverse: slack-cli returns newest-first; we want oldest-first for created_at ordering.
			for l, r := 0, len(msgs)-1; l < r; l, r = l+1, r-1 {
				msgs[l], msgs[r] = msgs[r], msgs[l]
			}
			return msgs
		}
	}
	return nil
}

// WriteContext appends active Slack channels, threads, and DM configs for contextID to w.
func (p *SlackProvider) WriteContext(ctx context.Context, w io.Writer, contextID string, startAt time.Time, _ bool) error {
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
