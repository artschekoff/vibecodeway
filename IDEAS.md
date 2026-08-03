# IDEAS — Development track

Product roadmap for winning **senior developers** and **team leads**. This is an
opinionated backlog, not a wishlist: every item names the persona, the pain, the
build, and why it drives adoption. Ordered by leverage, not effort.

---

## Strategic thesis

Context Consolidator is a broad suite (aggregation + AI + editor + terminal +
review). Breadth is a liability for adoption — pros adopt a tool that nails **one
painful job in the first five minutes**, then discover the rest.

**The wedge:** be *the context layer for AI coding agents.* Everyone runs Claude
Code / Cursor / Codex; every agent starts blind and every dev re-pastes the same
Jira ticket + Slack decision + PR feedback. We already assemble all of it and can
pipe it to an agent. Make that the front door; everything else is supporting cast.

**Two personas, two jobs:**
- **Senior dev** — "get me and my agent grounded, then get out of the way." Cares
  about focus, low friction, code-aware AI, keyboard-first flow.
- **Team lead** — "who's blocked, what needs review, is the sprint on track." Cares
  about triage, visibility, handoffs, and not being a bottleneck.

We keep them in one product because the lead's context *is* the dev's context —
the same object, two lenses.

---

## Track A — Senior developers

### A1. Agent grounding as the front door  ⭐ (highest leverage)
- **Pain:** every AI-agent session starts blind; devs manually paste ticket +
  thread + PR comments + file context.
- **Build:** `context-consolidator prime <ticket|PR|branch>` → one command emits (or
  auto-pipes into the active terminal agent) a ready bundle: ticket + comments +
  linked Slack decisions + PR review state + changed files. Make the CLI the hero;
  GUI optional. Extend the existing `Ask AI → terminal` flow to a full "prime the
  agent" action.
- **Why it wins:** this is the viral loop — a senior dev forwards *"grounds your
  agent in the real task in one command"* to the whole team.

### A2. Ticket → code-aware plan breakdown
- **Pain:** the gap between "here's the Jira ticket" and "here are the concrete
  steps in *this* codebase" is where agents flail and devs babysit.
- **Build:** feed ticket + repo file tree + relevant file contents (we already have
  `GetRepoFileTree` / `ReadRepoFiles`) to the AI layer → a checklist of concrete,
  file-referencing steps. Each step becomes an "Ask AI" prompt or a terminal task.
- **Why it wins:** best demo in the product; bridges the AI layer and the editor;
  turns snapshots from "meeting notes" into an executable plan.

### A3. Codebase-aware context (make snapshots true)
- **Pain:** AI snapshots/estimates reason only over tickets/threads, not the actual
  code — so they're plausible but not *engineering* truth.
- **Build:** let event-extraction / snapshot / estimation optionally read repo state
  (diff vs base, changed files, recent commits) for the linked worktree.
- **Why it wins:** "the AI actually knows what the code looks like" — credibility
  jump that separates this from every ticket-summarizer.

### A4. Native MCP server
- **Pain:** CLI glue is still glue; devs want agents to pull context *natively*.
- **Build:** expose contexts as an MCP server so Claude/Cursor/Codex read a context
  as a first-class tool — no copy-paste, no shell.
- **Why it wins:** rides the standard everyone's adopting; zero-friction integration
  is what makes a tool "always on."

### A5. Frictionless install & 60-second aha
- **Pain:** `go install wails` + compile is a cliff; requiring acli+gh+Slack+AI keys
  before *any* value kills trial.
- **Build:** signed DMG + Homebrew cask; a zero-integration first run ("open a repo,
  curate agent hunks, prime an agent") that delivers value before asking for creds.
- **Why it wins:** popularity is gated on time-to-first-aha, not feature count.

### A6. Keyboard-first everywhere
- **Pain:** senior devs live on the keyboard; mouse-driven flows lose them.
- **Build:** command palette (⌘K) across the app; extend the cmux terminal keymap
  to global navigation (jump to context, review, file); fully rebindable.
- **Why it wins:** table-stakes for the "pro tool" perception; compounds daily use.

---

## Track B — Team leads

### B1. "What needs me now" triage view  ⭐
- **Pain:** the lead's day is triage — reviews owed, blockers, re-review requests,
  changes-requested — scattered across tabs.
- **Build:** collapse My Board + Pending Reviews into a single prioritized inbox:
  *your* action items across all contexts, ranked, with the new **re-review** and
  unresolved-comment signals front and center.
- **Why it wins:** a lead opens a tool daily only if it answers "what should I do
  next" without digging. This is the retention hook.

### B2. Blocker & risk board
- **Pain:** blockers already exist as extracted *events* but have no surface; leads
  can't see what's stuck.
- **Build:** a dedicated blocker/risk lane aggregating `blocker` / `open_question`
  events across contexts, with age and owner. Optional Slack/notification nudges.
- **Why it wins:** unblocking is the lead's highest-value act; make it a glance.

### B3. Sprint / status rollup
- **Pain:** "is the sprint on track" means opening Jira and eyeballing.
- **Build:** roll up workflow statuses + estimates + PR/CI state across a set of
  contexts into a one-screen health view (on-track / at-risk / blocked).
- **Why it wins:** turns the tool into the lead's standup and status-report source.

### B4. First-class handoff & delegation
- **Pain:** handoffs lose context; assignment docs are free-form blobs today.
- **Build:** upgrade Person Assignments into structured, code-aware task lists
  (from A2's breakdown), with a shareable handoff link/packet and an "assigned to
  me across all contexts" inbox on the dev side.
- **Why it wins:** clean delegation is the lead's leverage; closes the loop between
  lead (assigns) and dev (receives grounded, actionable work).

### B5. Team context sharing (the moat)
- **Pain:** contexts are single-player; the lead's assembled context can't reach the
  dev doing the work.
- **Build:** optional share/sync of a context (export bundle first; later a
  local-first sync layer) so the whole task — tickets, threads, review state, plan —
  moves as one object. Preserve local-first: opt-in, self-hostable.
- **Why it wins:** switching cost compounds when the team's *shared context graph*
  lives here. This is the durable moat — not any single feature.

---

## Positioning & moat (guardrails for the track)

- **Lead with the wedge, not the suite.** Headline: *"Your AI agents, grounded in
  the real task — Jira + Slack + review + code, one command."*
- **Local-first is the enterprise wedge.** "Your tickets, code, and creds never
  leave your machine" is a real buying reason where cloud dev tools get blocked.
  Every roadmap item must preserve it (sharing/sync stays opt-in / self-hostable).
- **The moat is being the neutral context hub** across tools *and* agents. Own the
  integration graph and the agent interface; features are copyable, the graph isn't.

## Sequencing (suggested)

1. **A1 + A5** — agent grounding front door + frictionless install. (Get to viral.)
2. **B1** — triage view. (Daily retention for leads.)
3. **A2 + A3** — plan breakdown + codebase-aware AI. (The killer demo + credibility.)
4. **A4** — MCP server. (Always-on integration.)
5. **B2–B5** — blocker board, rollup, handoff, team sharing. (Deepen the moat.)

## Anti-goals (say no to keep the wedge sharp)

- No generic project management (don't fight Jira/Linear head-on).
- No cloud-first data model that breaks the privacy story.
- No more integrations until the ones we have hit 60-second aha.
- No feature that doesn't serve either "ground my agent" or "tell me what needs me."
