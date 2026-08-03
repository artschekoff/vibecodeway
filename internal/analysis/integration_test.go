package analysis_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/artschekoff/context-consolidator/internal/analysis"
	jiraservice "github.com/artschekoff/context-consolidator/internal/jira/service"
	jirastore "github.com/artschekoff/context-consolidator/internal/jira/store"
	"github.com/artschekoff/context-consolidator/internal/openai"
	slackstore "github.com/artschekoff/context-consolidator/internal/slack/store"
	appstore "github.com/artschekoff/context-consolidator/internal/store"
	vcsservice "github.com/artschekoff/context-consolidator/internal/vcs/service"
	vcsstore "github.com/artschekoff/context-consolidator/internal/vcs/store"
)

// projectRoot walks up from this file's location to find the .env file.
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func loadEnv(t *testing.T) {
	t.Helper()
	root := projectRoot()
	if root == "" {
		t.Skip("could not locate project root for .env loading")
	}
	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		t.Skipf(".env file not found at %s — create it with OPENAI_API_KEY=sk-...", envPath)
	}
	if err := godotenv.Load(envPath); err != nil {
		t.Skipf("failed to load .env: %v", err)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY is empty in .env — fill it in to run AI integration tests")
	}
}

func openTestStore(t *testing.T) *appstore.Store {
	t.Helper()
	s, err := appstore.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err, "open test store")
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestAnalysis_FullPipeline seeds one Jira issue, one Slack thread, and one GitHub PR
// then calls AnalyzeContext and GenerateHandoffPacket against the real OpenAI API.
func TestAnalysis_FullPipeline(t *testing.T) {
	t.Skip("integration test — requires live API keys")
	loadEnv(t)

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	s := openTestStore(t)
	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())
	vcsStore := vcsstore.NewStore(s.DB())

	// --- seed org + context ---
	org, err := s.CreateOrganization(ctx, "acme")
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "PROJ", org.ID)
	require.NoError(t, err)
	contextID := ctxRow.ID

	// --- seed a Jira issue ---
	_, err = jiraRepo.UpsertIssue(ctx, contextID,
		"PROJ-42",
		"Implement user authentication with OAuth2",
		"In Progress",
		"alice",
		"Story",
		"Users need to log in via Google OAuth2. "+
			"The system must not store passwords. "+
			"Session tokens expire after 24 hours. "+
			"Admin users bypass MFA for internal tools.",
		time.Now().Add(-48*time.Hour).UTC().Format(time.RFC3339),
		"",
		"",
	)
	require.NoError(t, err)

	// --- seed a Slack thread ---
	_, err = slackRepo.UpsertSlackThread(ctx, contextID, slackstore.UpsertSlackThreadArgs{
		Workspace:   "acme",
		ChannelID:   "C-GENERAL",
		ThreadTS:    "1715000000.000001",
		ChannelName: "general",
		Permalink:   "https://acme.slack.com/archives/C-GENERAL/p1715000000000001",
		SearchQuery: "oauth",
		Content: `alice [2026-05-10 09:15]: Hey team, we decided OAuth2 via Google is the only supported login.
No email/password at all — product wants zero credential storage.
bob [2026-05-10 09:22]: Confirmed. Legal also requires audit logs for every login event.
alice [2026-05-10 09:30]: Good point. Let's make sure the login audit table is append-only.
charlie [2026-05-10 10:05]: Also — mobile clients need a refresh-token flow since sessions expire at 24h.`,
	})
	require.NoError(t, err)

	// --- seed a GitHub PR with a review comment ---
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "acme-org", "backend")
	require.NoError(t, err)

	branch, err := vcsStore.Branches.AddBranch(ctx, contextID, repo.ID, "feat/oauth2", "OAuth2 branch")
	require.NoError(t, err)

	prResult, err := vcsStore.PullRequests.UpsertPR(ctx,
		repo.ID, branch.ID, 17,
		"feat: add OAuth2 login flow",
		"alice",
		"open",
		"changes_requested",
		"https://github.com/acme-org/backend/pull/17",
		"feat/oauth2",
		"mergeable",
		"passed",
		false,
		time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-2*time.Hour).UTC().Format(time.RFC3339),
	)
	require.NoError(t, err)

	_, _, err = vcsStore.PullRequests.UpsertComment(ctx,
		prResult.ID,
		"gh-comment-9901",
		nil,
		"review",
		"bob",
		"The refresh token endpoint is missing rate-limiting. "+
			"If we allow unlimited refresh attempts we risk credential stuffing. "+
			"Also the audit log write should be transactional with the session creation — "+
			"if audit write fails we should roll back the session.",
		nil, nil, false, nil,
		time.Now().Add(-1*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-1*time.Hour).UTC().Format(time.RFC3339),
	)
	require.NoError(t, err)

	// --- build service under test ---
	jiraProv := jiraservice.NewJiraProvider(jiraRepo, nil, logger, nil)
	slackProv := jiraservice.NewSlackProvider(slackRepo, &noopSlackRunner{}, logger, nil)
	vcsProv := vcsservice.NewVCSProvider(vcsStore, logger, nil)

	jiraSvc := jiraservice.NewContextService(
		s, nil, jiraRepo, slackRepo, &noopSlackRunner{}, logger,
		jiraProv, slackProv, vcsProv,
	)

	aiClient := openai.NewClient(s, logger)
	svc := analysis.NewService(s, jiraSvc, aiClient, logger)

	// --- AnalyzeContext ---
	t.Log("running AnalyzeContext (real OpenAI call)…")
	snapshot, err := svc.AnalyzeContext(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	t.Logf("snapshot status:  %s", snapshot.Status)
	t.Logf("current step:     %s (confidence: %s)", snapshot.CurrentStepDesc, snapshot.CurrentStepConfidence)
	t.Logf("operational:\n%s", snapshot.SectionOperational)
	t.Logf("narrative:\n%s", snapshot.SectionNarrative)
	t.Logf("engineering:\n%s", snapshot.SectionEngineering)
	t.Logf("next short:  %s", snapshot.AINextStepsShort)
	t.Logf("next mid:    %s", snapshot.AINextStepsMid)
	t.Logf("next long:   %s", snapshot.AINextStepsLong)

	assert.NotEmpty(t, snapshot.Status)
	assert.NotEmpty(t, snapshot.SectionOperational)
	assert.NotEmpty(t, snapshot.SectionNarrative)
	assert.NotEmpty(t, snapshot.CurrentStepDesc)

	// Snapshot must have been persisted.
	stored, err := s.GetSnapshot(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, snapshot.Status, stored.Status)

	// Events extracted from the 3 sources.
	events, err := s.ListEvents(ctx, contextID)
	require.NoError(t, err)
	assert.NotEmpty(t, events, "expected at least one event extracted from Jira/Slack/GitHub content")
	t.Logf("extracted %d events", len(events))
	for _, e := range events {
		t.Logf("  [%s] %s — %s", e.Type, e.SourceType, e.Summary)
	}

	// --- GenerateHandoffPacket ---
	t.Log("running GenerateHandoffPacket (real OpenAI call)…")
	packet, err := svc.GenerateHandoffPacket(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, packet)

	t.Logf("packet id:    %s", packet.PacketID)
	t.Logf("global conf:  %.2f", packet.Summary.GlobalConfidence)
	t.Logf("requirements: %d", len(packet.Requirements))
	for _, r := range packet.Requirements {
		t.Logf("  [%s] %s — %s (conf %.2f)", r.RequirementType, r.RequirementID, r.Title, r.Confidence)
	}
	if packet.HandoffCandidate != nil {
		t.Logf("handoff candidate: %s (conf %.2f)", packet.HandoffCandidate.HandoffTitle, packet.HandoffCandidate.Confidence)
	}

	assert.NotEmpty(t, packet.PacketID)
	assert.NotEmpty(t, packet.GeneratedAt)
	assert.NotEmpty(t, packet.Requirements, "expected at least one inferred requirement")
	assert.Greater(t, packet.Summary.GlobalConfidence, 0.0)

	// Packet must have been persisted.
	latest, err := s.GetLatestPacket(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, packet.PacketID, latest.PacketID)
}

// TestAnalysis_RealJiraTicket runs the full pipeline against real content from JH-477 / JH-2210
// and writes a human-readable report to docs/test.md for review.
func TestAnalysis_RealJiraTicket(t *testing.T) {
	t.Skip("integration test — requires live API keys")
	loadEnv(t)

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	s := openTestStore(t)
	jiraRepo := jirastore.NewRepo(s.DB())
	slackRepo := slackstore.NewRepo(s.DB())
	vcsStore := vcsstore.NewStore(s.DB())

	org, err := s.CreateOrganization(ctx, "juna-health")
	require.NoError(t, err)

	ctxRow, err := s.CreateContext(ctx, "JH-2210", org.ID)
	require.NoError(t, err)
	contextID := ctxRow.ID

	// Parent story JH-477 with full description from Jira.
	_, err = jiraRepo.UpsertIssue(ctx, contextID,
		"JH-477",
		"[Practice] Pro Paywall -- Referral Missed Opportunities Blur",
		"In Progress",
		"artem@jirohealth.com",
		"Story",
		`v1 implementation agreed in the Referral pro-paywall implementation meeting (May 5, 2026).
Full blur of the referral table for non-pro users — the first row will NOT be exposed in v1 (departure from the current Figma).

Goal: Gate the Referral Analysis missed-opportunities table behind the Pro subscription.
Non-pro users see top-line metrics (Missed Referrals, Revenue Missed) plus a fully blurred provider table with mock data and an upgrade CTA.
Pro users see the real provider data.

In scope (v1):
- Backend pro-flag check on the referral missed-opportunities endpoint
- Server returns empty array + isMocked/isHidden status flag for non-pro users (no real data leaves the server)
- Frontend reads the flag and injects static client-side mock data for the table rows
- CSS blur applied to all data rows
- Click handler disabled on mock rows — the detail drawer must NOT open
- Upgrade CTA button ("Get referral data") wired to the existing pro-paywall upgrade modal Timur is building
- Top-level metrics (Missed Referrals count, Revenue Missed) remain visible to non-pro users

Out of scope (handled in [Cross-Pod] Pro-Paywall Features [GA launch] epic):
- Showing the first row with real data (rejected for v1 — security risk via repeated refetch)
- Clickable mock first row that opens a drill-down drawer with mock data
- "What do I get" / "What premium gives you" button + modal
- Video walkthrough showing pro-only experience
- Abstract / reusable pro-paywall middleware (deferred until coding pattern detection lands)

Backend (Artem):
- Add pro-subscription check on the referral missed-opportunities endpoint
- If non-pro: return empty array plus a status flag indicating the data is hidden/mocked. If pro: return real data unchanged
- Flag travels on the response payload (decided over user-profile placement so FE doesn't have to sync two sources)
- Default behavior when flag is absent: treat as non-pro (FE shows mock + blur)
- Implementation approach: simple per-endpoint filtering for v1. Do NOT build the abstract middleware now.

Frontend (Rustam):
- Read the isMocked/isHidden flag from the API response
- If flag is true: inject static mock data into the table and apply CSS blur over all rows
- Static mock data is held client-side — no DB storage required
- Add condition to ignore clicks on mock rows (drill-down drawer must not open)
- Wire "Get referral data" CTA to the existing upgrade modal (Timur's work, believed already on dev — confirm)
- Mock data content: Easter-egg / fun-but-realistic provider names and numbers. Rustam to draft and share with Liora for review.

Acceptance criteria:
1. Non-pro user loads the Referral Analysis page: top-line metrics render with real numbers; provider table renders fully blurred with mock data; upgrade CTA visible.
2. Non-pro user clicks any blurred row: nothing happens (no drawer opens, no navigation).
3. Non-pro user clicks "Get referral data": existing upgrade modal opens.
4. Network inspection on a non-pro session shows no real provider records in the API response (empty array + flag only).
5. Pro user loads the page: real provider table renders unblurred; row clicks open the detail drawer normally.
6. User who upgrades from non-pro to pro mid-session sees real data on next page load / data refetch.

Open questions / follow-ups:
- Confirm the upgrade-modal CTA wiring with Timur (believed done on dev).
- Mock data table content — Rustam to share draft for Liora's sign-off.
- Confirm exact name of the response flag (isMocked vs isHidden vs other) — Artem + Rustam.

Source: Referral pro-paywall implementation meeting — May 5, 2026 (Liora, Artem, Rustam, McLean).`,
		time.Now().Add(-7*24*time.Hour).UTC().Format(time.RFC3339),
		"",
		"",
	)
	require.NoError(t, err)

	// Subtask JH-2210 (API engineering scope).
	_, err = jiraRepo.UpsertIssue(ctx, contextID,
		"JH-2210",
		"[juna-api] - development (JH-477 - [Practice] Pro Paywall -- Referral Missed Opportunities Blur)",
		"In Progress",
		"artem@jirohealth.com",
		"Subtask",
		`Development tasks for juna-api scope.
Write engineering notes here. Update ticket to require configuration or DB schema change, setup instructions or test coverage requirement.
Move subtask to Ready For Development status to create github branch.
Acceptance Criteria: 1. [todo] define acceptance criteria for this engineering task.`,
		time.Now().Add(-5*24*time.Hour).UTC().Format(time.RFC3339),
		"JH-477",
		"",
	)
	require.NoError(t, err)

	// GitHub PR #221 for the API work.
	repo, err := vcsStore.Repos.EnsureRepo(ctx, "github", "juna-health", "juna-api")
	require.NoError(t, err)

	branch, err := vcsStore.Branches.AddBranch(ctx, contextID, repo.ID, "feat_JH-477_task_JH-2210_pr", "Pro paywall branch")
	require.NoError(t, err)

	prResult, err := vcsStore.PullRequests.UpsertPR(ctx,
		repo.ID, branch.ID, 221,
		"feat[2210]: practice pro paywall — referral widget missed-opportunities gate",
		"artschekoff",
		"open",
		"changes_requested",
		"https://github.com/juna-health/juna-api/pull/221",
		"feat_JH-477_task_JH-2210_pr",
		"mergeable",
		"passed",
		false,
		time.Now().Add(-3*24*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-12*time.Hour).UTC().Format(time.RFC3339),
	)
	require.NoError(t, err)

	_, _, err = vcsStore.PullRequests.UpsertComment(ctx,
		prResult.ID,
		"gh-review-pr221-1",
		nil,
		"review",
		"rustam",
		`The isMocked flag naming: should we align on isHidden instead since "mocked" implies the data format might differ from real data, but isHidden just signals visibility gating? Either works but let's pick one before merge so FE and BE are in sync.`,
		nil, nil, false, nil,
		time.Now().Add(-10*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-10*time.Hour).UTC().Format(time.RFC3339),
	)
	require.NoError(t, err)

	_, _, err = vcsStore.PullRequests.UpsertComment(ctx,
		prResult.ID,
		"gh-review-pr221-2",
		nil,
		"review",
		"mclean",
		`One edge case: what happens when a practice is pro but the referral data is genuinely empty (0 missed opportunities)? The FE needs to distinguish "empty because no data" from "empty because non-pro". Make sure the response shape communicates both clearly.`,
		nil, nil, false, nil,
		time.Now().Add(-8*time.Hour).UTC().Format(time.RFC3339),
		time.Now().Add(-8*time.Hour).UTC().Format(time.RFC3339),
	)
	require.NoError(t, err)

	// Build service under test.
	jiraProv := jiraservice.NewJiraProvider(jiraRepo, nil, logger, nil)
	slackProv := jiraservice.NewSlackProvider(slackRepo, &noopSlackRunner{}, logger, nil)
	vcsProv := vcsservice.NewVCSProvider(vcsStore, logger, nil)

	jiraSvc := jiraservice.NewContextService(
		s, nil, jiraRepo, slackRepo, &noopSlackRunner{}, logger,
		jiraProv, slackProv, vcsProv,
	)

	aiClient := openai.NewClient(s, logger)
	svc := analysis.NewService(s, jiraSvc, aiClient, logger)

	// Run AnalyzeContext.
	t.Log("running AnalyzeContext…")
	snapshot, err := svc.AnalyzeContext(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, snapshot)

	events, err := s.ListEvents(ctx, contextID)
	require.NoError(t, err)

	// Run GenerateHandoffPacket.
	t.Log("running GenerateHandoffPacket…")
	packet, err := svc.GenerateHandoffPacket(ctx, contextID)
	require.NoError(t, err)
	require.NotNil(t, packet)

	// Write full output to docs/test.md.
	root := projectRoot()
	require.NotEmpty(t, root, "could not locate project root")
	outPath := filepath.Join(root, "docs", "test.md")

	var md strings.Builder
	md.WriteString("# Analysis pipeline output — JH-2210 / JH-477\n\n")
	md.WriteString("_Generated by TestAnalysis_RealJiraTicket_\n\n")

	md.WriteString("---\n\n## Snapshot\n\n")
	md.WriteString("**Status:** `" + snapshot.Status + "`\n\n")
	md.WriteString("**Current step:** " + snapshot.CurrentStepDesc + " _(confidence: " + snapshot.CurrentStepConfidence + ")_\n\n")

	md.WriteString("### Operational\n\n")
	md.WriteString(snapshot.SectionOperational + "\n\n")

	md.WriteString("### Narrative\n\n")
	md.WriteString(snapshot.SectionNarrative + "\n\n")

	md.WriteString("### Engineering\n\n")
	md.WriteString(snapshot.SectionEngineering + "\n\n")

	md.WriteString("### AI next steps\n\n")
	md.WriteString("**Today:** " + snapshot.AINextStepsShort + "\n\n")
	md.WriteString("**This week:** " + snapshot.AINextStepsMid + "\n\n")
	md.WriteString("**This month:** " + snapshot.AINextStepsLong + "\n\n")

	md.WriteString("---\n\n## Extracted events\n\n")
	for _, e := range events {
		md.WriteString("- **[" + e.Type + "]** `" + e.SourceType + "` — " + e.Summary + "\n")
	}
	md.WriteString("\n")

	md.WriteString("---\n\n## Inferred requirements packet\n\n")
	if packet.Summary.BusinessProblem != nil {
		md.WriteString("**Business problem:** " + *packet.Summary.BusinessProblem + "\n\n")
	}
	if packet.Summary.ProductOrInitiative != nil {
		md.WriteString("**Initiative:** " + *packet.Summary.ProductOrInitiative + "\n\n")
	}
	md.WriteString("**Global confidence:** " + fmt.Sprintf("%.0f%%", packet.Summary.GlobalConfidence*100) + "\n\n")

	if len(packet.Summary.DesiredOutcomes) > 0 {
		md.WriteString("**Desired outcomes:**\n")
		for _, o := range packet.Summary.DesiredOutcomes {
			md.WriteString("- " + o + "\n")
		}
		md.WriteString("\n")
	}
	if len(packet.Summary.NonGoals) > 0 {
		md.WriteString("**Non-goals:**\n")
		for _, g := range packet.Summary.NonGoals {
			md.WriteString("- " + g + "\n")
		}
		md.WriteString("\n")
	}

	md.WriteString("### Requirements\n\n")
	for _, r := range packet.Requirements {
		md.WriteString("#### " + r.RequirementID + " — " + r.Title + "\n\n")
		md.WriteString("| Field | Value |\n|---|---|\n")
		md.WriteString("| Type | `" + r.RequirementType + "` |\n")
		md.WriteString("| Status | `" + r.Status + "` |\n")
		md.WriteString("| Priority | `" + r.Priority + "` |\n")
		md.WriteString("| Confidence | " + fmt.Sprintf("%.0f%%", r.Confidence*100) + " |\n\n")
		md.WriteString("**Statement:** " + r.Statement + "\n\n")
		if r.BusinessValue != nil {
			md.WriteString("**Business value:** " + *r.BusinessValue + "\n\n")
		}
		if len(r.Evidence) > 0 {
			md.WriteString("**Evidence:**\n")
			for _, ev := range r.Evidence {
				md.WriteString("- _\"" + ev.Quote + "\"_")
				if ev.Rationale != nil {
					md.WriteString(" — " + *ev.Rationale)
				}
				md.WriteString("\n")
			}
			md.WriteString("\n")
		}
		if len(r.Relations) > 0 {
			md.WriteString("**Relations:**\n")
			for _, rel := range r.Relations {
				md.WriteString("- `" + string(rel.RelationType) + "` → " + rel.TargetRequirementID + "\n")
			}
			md.WriteString("\n")
		}
	}

	if packet.HandoffCandidate != nil {
		hc := packet.HandoffCandidate
		md.WriteString("---\n\n## Handoff candidate\n\n")
		md.WriteString("**Title:** " + hc.HandoffTitle + "\n\n")
		md.WriteString("**Objective:** " + hc.Objective + "\n\n")
		md.WriteString("**Confidence:** " + fmt.Sprintf("%.0f%%", hc.Confidence*100) + "\n\n")
		if len(hc.KeyBusinessRules) > 0 {
			md.WriteString("**Key business rules:**\n")
			for _, r := range hc.KeyBusinessRules {
				md.WriteString("- " + r + "\n")
			}
			md.WriteString("\n")
		}
		if len(hc.AcceptanceCriteria) > 0 {
			md.WriteString("**Acceptance criteria:**\n")
			for _, ac := range hc.AcceptanceCriteria {
				md.WriteString("- " + ac + "\n")
			}
			md.WriteString("\n")
		}
		if len(hc.OpenQuestions) > 0 {
			md.WriteString("**Open questions:**\n")
			for _, q := range hc.OpenQuestions {
				md.WriteString("- " + q + "\n")
			}
			md.WriteString("\n")
		}
	}

	if packet.ExtractionNotes != nil && *packet.ExtractionNotes != "" {
		md.WriteString("---\n\n## Extraction notes\n\n" + *packet.ExtractionNotes + "\n")
	}

	require.NoError(t, os.WriteFile(outPath, []byte(md.String()), 0644))
	t.Logf("wrote report to %s", outPath)
}

// noopSlackRunner is a slack runner that returns empty output (no Slack API calls in tests).
type noopSlackRunner struct{}

func (r *noopSlackRunner) Run(_ context.Context, _ []string, _ []string) (string, error) {
	return "", nil
}
