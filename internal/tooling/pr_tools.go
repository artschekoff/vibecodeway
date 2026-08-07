package tooling

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/artschekoff/context-consolidator/internal/vcs"
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
		"number":       p.Number,
		"title":        p.Title,
		"author":       p.Author,
		"state":        p.State,
		"reviewStatus": p.ReviewStatus,
		"url":          p.URL,
		"headBranch":   p.HeadBranch,
		"mergeable":    p.Mergeable,
		"ciStatus":     p.CIStatus,
		"reviewers":    reviewers,
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
}
