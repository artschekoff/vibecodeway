package tooling

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	vcsstore "github.com/artschekoff/vibecodeway/internal/vcs/store"
)

// splitRepo parses an "owner/name" repo reference.
func splitRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(strings.TrimSpace(repo), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo must be in 'owner/name' form, got %q", repo)
	}
	return parts[0], parts[1], nil
}

// intArg reads an integer argument, tolerating JSON numbers (float64) and strings.
func intArg(args map[string]any, k string) int {
	switch v := args[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

// prBrief is the compact PR view returned to the model.
func prBrief(p vcs.PRData) map[string]any {
	reviewers := make([]map[string]any, 0, len(p.Reviewers))
	for _, rv := range p.Reviewers {
		reviewers = append(reviewers, map[string]any{"login": rv.Login, "state": rv.State, "submittedAt": rv.SubmittedAt})
	}
	return map[string]any{
		"number":             p.Number,
		"title":              p.Title,
		"author":             p.Author,
		"state":              p.State,
		"reviewStatus":       p.ReviewStatus,
		"url":                p.URL,
		"headBranch":         p.HeadBranch,
		"mergeable":          p.Mergeable,
		"ciStatus":           p.CIStatus,
		"mergeStateStatus":   p.MergeStateStatus,
		"updateBranchNeeded": p.MergeStateStatus == "BEHIND",
		"reviewers":          reviewers,
	}
}

// storedBrief is the compact view of a stored PR snapshot (the lazy-load cache).
func storedBrief(rr vcsstore.ReviewRequest) map[string]any {
	return map[string]any{
		"number":             rr.PRNumber,
		"title":              rr.PRTitle,
		"author":             rr.PRAuthor,
		"reviewStatus":       rr.ReviewStatus,
		"url":                rr.PRURL,
		"headBranch":         rr.HeadBranch,
		"mergeable":          rr.Mergeable,
		"ciStatus":           rr.CIStatus,
		"mergeStateStatus":   rr.MergeStateStatus,
		"updateBranchNeeded": rr.MergeStateStatus == "BEHIND",
		"unresolvedComments": rr.UnresolvedCount,
		"myReviewState":      rr.MyReviewState,
		"kind":               rr.Kind,
		"fetchedAt":          rr.FetchedAt,
	}
}

func registerPRTools(r *Registry, d Deps) {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	numProp := func(desc string) map[string]any {
		return map[string]any{"type": "number", "description": desc}
	}

	r.Register(Tool{
		Name:        "list_review_requests",
		Description: "List open pull requests in a repo that are awaiting review from a user. Defaults to the authenticated user (@me); pass a login to see another user's requested reviews.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": strProp("repository as 'owner/name'"),
				"user": strProp("GitHub login to check (optional, default @me)"),
			},
			"required": []string{"repo"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			prs, err := d.VCS.ListReviewRequestsForUser(ctx, owner, name, strArg(args, "user"))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(prs))
			for _, p := range prs {
				out = append(out, prBrief(p))
			}
			return map[string]any{"count": len(out), "pullRequests": out}, nil
		},
	})

	r.Register(Tool{
		Name:        "list_changes_requested",
		Description: "List open pull requests in a repo where a user left a 'changes requested' review. Defaults to the authenticated user (@me).",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": strProp("repository as 'owner/name'"),
				"user": strProp("GitHub login to check (optional, default @me)"),
			},
			"required": []string{"repo"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			prs, err := d.VCS.ListChangesRequestedForUser(ctx, owner, name, strArg(args, "user"))
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(prs))
			for _, p := range prs {
				out = append(out, prBrief(p))
			}
			return map[string]any{"count": len(out), "pullRequests": out}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_pr",
		Description: "Get one pull request's metadata (title, author, state, review status, CI status, mergeability, reviewers) by number.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":   strProp("repository as 'owner/name'"),
				"number": numProp("the pull request number"),
			},
			"required": []string{"repo", "number"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			num := intArg(args, "number")
			if num <= 0 {
				return nil, fmt.Errorf("number is required")
			}
			pr, err := d.VCS.GetPRByNumber(ctx, owner, name, num)
			if err != nil {
				return nil, err
			}
			return prBrief(*pr), nil
		},
	})

	r.Register(Tool{
		Name:        "get_pr_comments",
		Description: "List the comments on a pull request (conversation, review bodies, and inline review comments) with author and body.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":   strProp("repository as 'owner/name'"),
				"number": numProp("the pull request number"),
			},
			"required": []string{"repo", "number"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			num := intArg(args, "number")
			if num <= 0 {
				return nil, fmt.Errorf("number is required")
			}
			comments, err := d.VCS.GetPRComments(ctx, owner, name, num)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(comments))
			for _, c := range comments {
				out = append(out, map[string]any{
					"author":     c.Author,
					"type":       c.CommentType,
					"body":       truncate(c.Body),
					"isResolved": c.IsResolved,
					"createdAt":  c.CreatedAt,
				})
			}
			return map[string]any{"count": len(out), "comments": out}, nil
		},
	})

	registerWritePRTools(r, d, strProp, numProp)
	registerPRStateTools(r, d, strProp, numProp)
}

// registerPRStateTools adds the lazy-load PR tools: reads default to the app's
// stored snapshot (vcs_review_requests); refreshes pull the latest state from
// GitHub and write it back, so the agent works against app state rather than
// hitting GitHub on every call. Requires the VCS store.
func registerPRStateTools(r *Registry, d Deps, strProp, numProp func(string) map[string]any) {
	if d.VCSStore == nil {
		return
	}
	boolProp := func(desc string) map[string]any {
		return map[string]any{"type": "boolean", "description": desc}
	}

	// findRepo resolves a tracked repo row without creating one — the snapshot
	// cache must never implicitly add repos to the Repositories tab.
	findRepo := func(ctx context.Context, owner, name string) *vcsstore.VCSRepo {
		repos, err := d.VCSStore.Repos.ListRepos(ctx, "github")
		if err != nil {
			return nil
		}
		for i := range repos {
			if strings.EqualFold(repos[i].Owner, owner) && strings.EqualFold(repos[i].Name, name) {
				return &repos[i]
			}
		}
		return nil
	}

	// isMe reports whether login refers to the authenticated user.
	isMe := func(ctx context.Context, login string) (string, bool) {
		me, err := d.VCS.GetAuthUser(ctx)
		if login == "" || login == "@me" {
			return me, err == nil
		}
		return me, err == nil && strings.EqualFold(login, me)
	}

	// upsertSnapshot writes one PR's latest state into the store, preserving an
	// existing row's kind (new rows for own PRs are kind "authored").
	upsertSnapshot := func(ctx context.Context, repoID string, p vcs.PRData, kind string) {
		_, _ = d.VCSStore.ReviewRequests.UpsertReviewRequest(ctx,
			repoID, p.Number, p.Title, p.Author, p.URL,
			p.ReviewStatus, p.HeadBranch, p.CreatedAt, marshalReviewersJSON(p.Reviewers),
			p.Mergeable, kind, p.CIStatus, p.MergeStateStatus)
		if p.State == "merged" || p.State == "closed" {
			_ = d.VCSStore.ReviewRequests.SetReviewStatus(ctx, repoID, p.Number, p.State)
		}
	}

	// storedRow finds a snapshot row by PR number; nil when not cached.
	storedRow := func(ctx context.Context, repoID string, number int) *vcsstore.ReviewRequest {
		rows, err := d.VCSStore.ReviewRequests.ListReviewRequests(ctx, repoID)
		if err != nil {
			return nil
		}
		for i := range rows {
			if rows[i].PRNumber == number {
				return &rows[i]
			}
		}
		return nil
	}

	r.Register(Tool{
		Name:        "list_author_prs",
		Description: "List a user's open pull requests in a repo. Defaults to the authenticated user (@me) and to the app's last-synced snapshot (lazy load); pass refresh=true to fetch the latest state from GitHub and update the snapshot first. Other users' PRs are always fetched live and not cached.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":    strProp("repository as 'owner/name'"),
				"user":    strProp("GitHub login of the PR author (optional, default @me)"),
				"refresh": boolProp("fetch the latest state from GitHub and update the snapshot before answering (default false)"),
			},
			"required": []string{"repo"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			login := strArg(args, "user")
			me, mine := isMe(ctx, login)
			repo := findRepo(ctx, owner, name)

			// Cached read: own PRs, tracked repo, no refresh requested.
			if mine && repo != nil && !boolArg(args, "refresh") {
				rows, err := d.VCSStore.ReviewRequests.ListReviewRequests(ctx, repo.ID)
				if err != nil {
					return nil, err
				}
				out := make([]map[string]any, 0)
				for _, rr := range rows {
					if strings.EqualFold(rr.PRAuthor, me) && rr.ReviewStatus != "merged" && rr.ReviewStatus != "closed" {
						out = append(out, storedBrief(rr))
					}
				}
				if len(out) > 0 {
					return map[string]any{"source": "snapshot", "count": len(out), "pullRequests": out}, nil
				}
				// Empty snapshot → fall through to a live fetch (first lazy load).
			}

			prs, err := d.VCS.ListAuthoredPRs(ctx, owner, name, login)
			if err != nil {
				return nil, err
			}
			out := make([]map[string]any, 0, len(prs))
			for _, p := range prs {
				if mine && repo != nil {
					upsertSnapshot(ctx, repo.ID, p, "authored")
					if rr := storedRow(ctx, repo.ID, p.Number); rr != nil {
						out = append(out, storedBrief(*rr))
						continue
					}
				}
				out = append(out, prBrief(p))
			}
			return map[string]any{"source": "github", "cached": mine && repo != nil, "count": len(out), "pullRequests": out}, nil
		},
	})

	r.Register(Tool{
		Name:        "get_pr_status",
		Description: "Get one pull request's status (review decision, update-branch need, CI, unresolved comments) from the app's last-synced snapshot (lazy load). Pass refresh=true to fetch the latest state from GitHub, update the snapshot, and answer from it. An uncached PR is fetched automatically.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":    strProp("repository as 'owner/name'"),
				"number":  numProp("the pull request number"),
				"refresh": boolProp("fetch the latest state from GitHub and update the snapshot before answering (default false)"),
			},
			"required": []string{"repo", "number"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, err := splitRepo(strArg(args, "repo"))
			if err != nil {
				return nil, err
			}
			num := intArg(args, "number")
			if num <= 0 {
				return nil, fmt.Errorf("number is required")
			}
			repo := findRepo(ctx, owner, name)
			var cached *vcsstore.ReviewRequest
			if repo != nil {
				cached = storedRow(ctx, repo.ID, num)
			}
			if cached != nil && !boolArg(args, "refresh") {
				return map[string]any{"source": "snapshot", "pullRequest": storedBrief(*cached)}, nil
			}

			pr, err := d.VCS.GetPRByNumber(ctx, owner, name, num)
			if err != nil {
				return nil, err
			}
			if pr == nil {
				return nil, fmt.Errorf("PR #%d not found in %s/%s", num, owner, name)
			}
			me, _ := d.VCS.GetAuthUser(ctx)
			// Cache only PRs already snapshotted or authored by the user, so the
			// Code Reviews tab is not polluted with arbitrary foreign PRs.
			if repo != nil && (cached != nil || strings.EqualFold(pr.Author, me)) {
				kind := "authored"
				if cached != nil {
					kind = cached.Kind
				}
				upsertSnapshot(ctx, repo.ID, *pr, kind)
				if count, err := d.VCS.CountUnresolvedThreads(ctx, owner, name, num); err == nil {
					_ = d.VCSStore.ReviewRequests.UpdateUnresolvedCount(ctx, repo.ID, num, count)
				}
				if rr := storedRow(ctx, repo.ID, num); rr != nil {
					return map[string]any{"source": "github", "cached": true, "pullRequest": storedBrief(*rr)}, nil
				}
			}
			brief := prBrief(*pr)
			if count, err := d.VCS.CountUnresolvedThreads(ctx, owner, name, num); err == nil {
				brief["unresolvedComments"] = count
			}
			return map[string]any{"source": "github", "cached": false, "pullRequest": brief}, nil
		},
	})
}

// marshalReviewersJSON serializes reviewer decisions for the snapshot row.
func marshalReviewersJSON(rvs []vcs.ReviewerDecision) string {
	if len(rvs) == 0 {
		return "[]"
	}
	b, err := json.Marshal(rvs)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// registerWritePRTools adds the PR write actions (approve / request changes / comment).
func registerWritePRTools(r *Registry, d Deps, strProp, numProp func(string) map[string]any) {
	// prNum resolves the required repo + number for a write action.
	prNum := func(args map[string]any) (owner, name string, num int, err error) {
		owner, name, err = splitRepo(strArg(args, "repo"))
		if err != nil {
			return "", "", 0, err
		}
		num = intArg(args, "number")
		if num <= 0 {
			return "", "", 0, fmt.Errorf("number is required")
		}
		return owner, name, num, nil
	}
	writeParams := func(bodyDesc string, bodyRequired bool) map[string]any {
		req := []string{"repo", "number"}
		if bodyRequired {
			req = append(req, "body")
		}
		return map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":   strProp("repository as 'owner/name'"),
				"number": numProp("the pull request number"),
				"body":   strProp(bodyDesc),
			},
			"required": req,
		}
	}

	r.Register(Tool{
		Name:        "approve_pr",
		Description: "Approve a pull request, optionally with a message. Submits an approving review as the authenticated user.",
		Write:       true,
		Params:      writeParams("approval message (optional)", false),
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, num, err := prNum(args)
			if err != nil {
				return nil, err
			}
			if err := d.VCS.ApprovePR(ctx, owner, name, num, strArg(args, "body")); err != nil {
				return nil, err
			}
			return map[string]any{"status": "approved", "number": num}, nil
		},
	})

	r.Register(Tool{
		Name:        "request_changes_pr",
		Description: "Request changes on a pull request with a required message explaining what needs to change. Submits a changes-requested review.",
		Write:       true,
		Params:      writeParams("what needs to change (required)", true),
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, num, err := prNum(args)
			if err != nil {
				return nil, err
			}
			body := strArg(args, "body")
			if strings.TrimSpace(body) == "" {
				return nil, fmt.Errorf("body is required to request changes")
			}
			if err := d.VCS.RequestChangesPR(ctx, owner, name, num, body); err != nil {
				return nil, err
			}
			return map[string]any{"status": "changes_requested", "number": num}, nil
		},
	})

	r.Register(Tool{
		Name:        "comment_pr",
		Description: "Leave a comment on a pull request (a review with no approval state). Requires a message.",
		Write:       true,
		Params:      writeParams("the comment text (required)", true),
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			owner, name, num, err := prNum(args)
			if err != nil {
				return nil, err
			}
			body := strArg(args, "body")
			if strings.TrimSpace(body) == "" {
				return nil, fmt.Errorf("body is required to comment")
			}
			if err := d.VCS.CommentPR(ctx, owner, name, num, body); err != nil {
				return nil, err
			}
			return map[string]any{"status": "commented", "number": num}, nil
		},
	})
}
