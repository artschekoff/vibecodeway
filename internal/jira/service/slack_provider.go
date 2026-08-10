package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/artschekoff/vibecodeway/internal/provider"
	slackstore "github.com/artschekoff/vibecodeway/internal/slack/store"
	appstore "github.com/artschekoff/vibecodeway/internal/store"
	"go.uber.org/zap"
)

// SlackProvider implements provider.Provider for Slack.
type SlackProvider struct {
	slackRepo *slackstore.Repo
	slack     SlackRunner
	logger    *zap.Logger
	msgWriter appstore.MessageWriter
}

// NewSlackProvider constructs a SlackProvider.
func NewSlackProvider(slackRepo *slackstore.Repo, slack SlackRunner, logger *zap.Logger, msgWriter appstore.MessageWriter) *SlackProvider {
	return &SlackProvider{slackRepo: slackRepo, slack: slack, logger: logger, msgWriter: msgWriter}
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
	if len(dmConfigs) > 0 {
		var newItems atomic.Int64
		dmJobs := make(chan slackstore.SlackDMConfig, len(dmConfigs))
		for _, cfg := range dmConfigs {
			dmJobs <- cfg
		}
		close(dmJobs)
		var dmWg sync.WaitGroup
		dmWg.Add(min(5, len(dmConfigs)))
		for range min(5, len(dmConfigs)) {
			go func() {
				defer dmWg.Done()
				for cfg := range dmJobs {
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
					if p.msgWriter != nil {
						externalID := "dm:" + cfg.Workspace
						authorID := p.msgWriter.ResolveAuthor(ctx, "slack", "")
						msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
							ContextID:  contextID,
							SourceID:   "slack",
							ExternalID: externalID,
							Title:      "DMs (" + cfg.Workspace + ")",
							Body:       content,
							RawAuthor:  "",
							AuthorID:   authorID,
							CreatedAt:  cfg.CreatedAt,
							UpdatedAt:  cfg.UpdatedAt,
						})
						if msgID != "" {
							_ = p.slackRepo.SetDMConfigMessageID(ctx, cfg.ID, msgID)
						}
					}
					newItems.Add(1)
				}
			}()
		}
		dmWg.Wait()
		stats.NewItems = int(newItems.Load())
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

	// Channel per-message records (slackChannelID set) are handled by refreshChannelMessages.
	// Old blobs (threadTS="") are cleaned up by refreshChannelMessages.
	var standaloneThreads []slackstore.SlackThread
	for _, t := range threads {
		if t.SlackChannelID == "" && t.ThreadTS != "" {
			standaloneThreads = append(standaloneThreads, t)
		}
	}
	if len(standaloneThreads) > 0 {
		var itemsRefreshed atomic.Int64
		tJobs := make(chan slackstore.SlackThread, len(standaloneThreads))
		for _, t := range standaloneThreads {
			tJobs <- t
		}
		close(tJobs)
		var tWg sync.WaitGroup
		tWg.Add(min(5, len(standaloneThreads)))
		for range min(5, len(standaloneThreads)) {
			go func() {
				defer tWg.Done()
				for t := range tJobs {
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
					thread, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
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
					if p.msgWriter != nil && thread != nil {
						authorID := p.msgWriter.ResolveAuthor(ctx, "slack", "")
						msgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
							ContextID:  contextID,
							SourceID:   "slack",
							ExternalID: t.ThreadTS,
							Title:      t.ChannelName,
							Body:       content,
							RawAuthor:  "",
							AuthorID:   authorID,
							CreatedAt:  slackTSToRFC3339(t.ThreadTS),
							UpdatedAt:  slackTSToRFC3339(t.ThreadTS),
						})
						if msgID != "" {
							_ = p.slackRepo.SetThreadMessageID(ctx, thread.ID, msgID)
						}
					}
					itemsRefreshed.Add(1)
				}
			}()
		}
		tWg.Wait()
		stats.ItemsRefreshed += int(itemsRefreshed.Load())
	}

	// ── Refresh channel messages: one slack_thread record per message ────────
	if len(channels) > 0 {
		var chRefreshed atomic.Int64
		chJobs := make(chan slackstore.SlackChannel, len(channels))
		for _, ch := range channels {
			chJobs <- ch
		}
		close(chJobs)
		var chWg sync.WaitGroup
		chWg.Add(min(4, len(channels)))
		for range min(4, len(channels)) {
			go func() {
				defer chWg.Done()
				for ch := range chJobs {
					chRefreshed.Add(int64(p.refreshChannelMessages(ctx, contextID, ch)))
				}
			}()
		}
		chWg.Wait()
		stats.ItemsRefreshed += int(chRefreshed.Load())
	}

	return stats, nil
}

// refreshChannelMessages fetches all messages for ch via search-channels, upserts one
// slack_thread record per top-level message, and deletes any old single-blob records.
// Returns the number of upserted records.
func (p *SlackProvider) refreshChannelMessages(ctx context.Context, contextID string, ch slackstore.SlackChannel) int {
	if ch.ChannelName == "" {
		p.logger.Warn("slack refresh: skipping channel with empty name",
			zap.String("channelID", ch.ChannelID))
		return 0
	}
	flags := []string{ch.Workspace, ch.ChannelName}
	if ch.StartFrom != "" {
		flags = append(flags, "--start-from", ch.StartFrom)
	}
	raw, err := p.slack.Run(ctx, []string{"search-channels"}, flags)
	if err != nil {
		p.logger.Warn("slack refresh: search-channels failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
		return 0
	}

	msgs := parseChannelMessages(raw, ch.ChannelID)
	if len(msgs) == 0 {
		p.logger.Warn("slack refresh: no messages returned for channel",
			zap.String("channel", ch.ChannelName))
		return 0
	}

	upserted := 0
	for _, msg := range msgs {
		if msg.RawTS == "" {
			p.logger.Warn("slack refresh: message missing rawTs, skipping",
				zap.String("channel", ch.ChannelName))
			continue
		}
		msgContent := formatSingleMessage(msg)
		msgThread, upsertErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
			SlackChannelID: ch.ID,
			Workspace:      ch.Workspace,
			ChannelID:      ch.ChannelID,
			ThreadTS:       msg.RawTS,
			ChannelName:    ch.ChannelName,
			Content:        msgContent,
		})
		if upsertErr != nil {
			p.logger.Warn("slack refresh: upsert message thread failed",
				zap.String("channel", ch.ChannelName), zap.String("ts", msg.RawTS), zap.Error(upsertErr))
			continue
		}
		if p.msgWriter != nil && msgThread != nil {
			rawAuthor := msg.UserID
			if msg.User != "" {
				rawAuthor = msg.User
			}
			authorID := p.msgWriter.ResolveAuthor(ctx, "slack", rawAuthor)
			topMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
				ContextID:  contextID,
				SourceID:   "slack",
				ExternalID: msg.RawTS,
				Title:      ch.ChannelName,
				Body:       msgContent,
				RawAuthor:  rawAuthor,
				AuthorID:   authorID,
				CreatedAt:  slackTSToRFC3339(msg.RawTS),
				UpdatedAt:  slackTSToRFC3339(msg.RawTS),
			})
			if topMsgID != "" {
				_ = p.slackRepo.SetThreadMessageID(ctx, msgThread.ID, topMsgID)
			}
		}
		upserted++
		for _, reply := range msg.Replies {
			if reply.RawTS == "" {
				continue
			}
			replyContent := formatSingleMessage(reply)
			replyThread, replyErr := p.slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
				SlackChannelID: ch.ID,
				Workspace:      ch.Workspace,
				ChannelID:      ch.ChannelID,
				ThreadTS:       reply.RawTS,
				ParentThreadTS: msg.RawTS,
				ChannelName:    ch.ChannelName,
				Content:        replyContent,
			})
			if replyErr != nil {
				p.logger.Warn("slack refresh: upsert reply thread failed",
					zap.String("channel", ch.ChannelName), zap.String("ts", reply.RawTS), zap.Error(replyErr))
			} else {
				if p.msgWriter != nil && replyThread != nil {
					rawAuthor := reply.UserID
					if reply.User != "" {
						rawAuthor = reply.User
					}
					authorID := p.msgWriter.ResolveAuthor(ctx, "slack", rawAuthor)
					replyMsgID, _ := p.msgWriter.UpsertMessage(ctx, appstore.UpsertMessageArgs{
						ContextID:  contextID,
						SourceID:   "slack",
						ExternalID: reply.RawTS,
						Title:      ch.ChannelName,
						Body:       replyContent,
						RawAuthor:  rawAuthor,
						AuthorID:   authorID,
						CreatedAt:  slackTSToRFC3339(reply.RawTS),
						UpdatedAt:  slackTSToRFC3339(reply.RawTS),
					})
					if replyMsgID != "" {
						_ = p.slackRepo.SetThreadMessageID(ctx, replyThread.ID, replyMsgID)
					}
				}
				upserted++
			}
		}
	}

	if upserted == 0 {
		return 0
	}

	// Delete old single-blob records (thread_ts="") now that per-message records exist.
	if err := p.slackRepo.DeleteChannelBlobs(ctx, contextID, ch.ChannelID); err != nil {
		p.logger.Warn("slack refresh: delete channel blobs failed",
			zap.String("channel", ch.ChannelName), zap.Error(err))
	}
	return upserted
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

// CollectItems returns one Item per Slack thread and DM config for global sorting.
func (p *SlackProvider) CollectItems(ctx context.Context, contextID string, startAt time.Time, _ bool) ([]provider.Item, error) {
	threads, err := p.slackRepo.ListSlackThreads(ctx, contextID, true)
	if err != nil {
		return nil, fmt.Errorf("loading threads: %w", err)
	}
	dmConfigs, err := p.slackRepo.ListSlackDMConfigs(ctx, contextID, true)
	if err != nil {
		return nil, fmt.Errorf("loading dm configs: %w", err)
	}
	return buildSlackItems(threads, dmConfigs, startAt), nil
}

func buildSlackItems(threads []slackstore.SlackThread, dmConfigs []slackstore.SlackDMConfig, startAt time.Time) []provider.Item {
	var items []provider.Item

	for _, t := range threads {
		ts := ParseTime(t.CreatedAt)
		if !startAt.IsZero() {
			// Treat unparseable timestamps as "include" to avoid silent data loss.
			if !ts.IsZero() && ts.Before(startAt) {
				continue
			}
		}
		if t.Content == "" {
			continue
		}
		chanLabel := t.ChannelName
		if chanLabel == "" {
			chanLabel = t.ChannelID
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · SLACK] #%s (%s)", FormatDateTime(t.CreatedAt), chanLabel, t.Workspace)
		if t.Permalink != "" {
			fmt.Fprintf(&buf, " — %s", t.Permalink)
		}
		fmt.Fprint(&buf, "\n")
		fmt.Fprint(&buf, t.Content)
		fmt.Fprint(&buf, "\n\n")
		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}

	for _, dm := range dmConfigs {
		if dm.Content == "" {
			continue
		}
		ts := ParseTime(dm.CreatedAt)
		if !startAt.IsZero() && !ts.IsZero() && ts.Before(startAt) {
			continue
		}
		label := fmt.Sprintf("DMs (%s)", dm.Workspace)
		if dm.StartFrom != "" {
			label += fmt.Sprintf(" since %s", dm.StartFrom)
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "[%s · SLACK] %s\n", FormatDateTime(dm.CreatedAt), label)
		fmt.Fprint(&buf, dm.Content)
		fmt.Fprint(&buf, "\n\n")
		items = append(items, provider.Item{CreatedAt: ts, Body: buf.String()})
	}
	return items
}

// slackTSToRFC3339 converts "1746441180.000000" → RFC3339. Falls back to now on parse failure.
func slackTSToRFC3339(ts string) string {
	parts := strings.SplitN(ts, ".", 2)
	if len(parts) == 0 {
		return time.Now().UTC().Format(time.RFC3339)
	}
	var sec int64
	if _, err := fmt.Sscanf(parts[0], "%d", &sec); err != nil {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return time.Unix(sec, 0).UTC().Format(time.RFC3339)
}
