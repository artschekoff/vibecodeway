package tooling

import (
	"context"
	"fmt"
	"strings"

	"github.com/artschekoff/vibecodeway/internal/vcs"
	"go.uber.org/zap"
)

// maxCommitHits caps how many commits list_commits returns, so a wide date range
// across many repositories cannot flood the model's context.
const maxCommitHits = 200

func registerCommitTools(r *Registry, d Deps) {
	strProp := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}

	r.Register(Tool{
		Name: "list_commits",
		Description: "Read git history from the tracked repositories' local checkouts, filtered by date. " +
			"Use this for any question about what work happened on a day or in a period " +
			"(\"what did I do last Friday\", \"what changed this week\"). " +
			"Resolve relative dates to absolute YYYY-MM-DD first — never pass a weekday name as a query.",
		Params: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"since":  strProp("start of the range, YYYY-MM-DD (inclusive)"),
				"until":  strProp("end of the range, YYYY-MM-DD (inclusive; optional, defaults to now)"),
				"author": strProp("substring matched against commit author name and email (optional; defaults to each repository's configured git user.email)"),
				"repo":   strProp("limit to one repository, as 'owner/name' (optional; default is every tracked repository)"),
			},
			"required": []string{"since"},
		},
		Handler: func(ctx context.Context, args map[string]any, _ Ctx) (any, error) {
			// Bare YYYY-MM-DD bounds are pinned to whole-day times inside
			// vcs.GitLog, so nothing here has to know about git's date parsing.
			since := strings.TrimSpace(strArg(args, "since"))
			if since == "" {
				return nil, fmt.Errorf("since is required, as YYYY-MM-DD")
			}
			until := strings.TrimSpace(strArg(args, "until"))
			author := strings.TrimSpace(strArg(args, "author"))
			repoFilter := strings.TrimSpace(strArg(args, "repo"))

			repos, err := d.Repos.ListReposWithLocations(ctx)
			if err != nil {
				return nil, err
			}

			commits := make([]map[string]any, 0, 32)
			noLocal := make([]string, 0)
			noIdentity := make([]string, 0)
			scanned := 0

			for _, repo := range repos {
				full := repo.FullName()
				if repoFilter != "" && !strings.EqualFold(full, repoFilter) {
					continue
				}
				if len(repo.Paths) == 0 {
					// Tracked but never cloned here: the model must be able to say
					// "I could not look in X" rather than imply no work happened.
					noLocal = append(noLocal, full)
					continue
				}
				// Dedupe by hash: two registered paths are usually two clones of
				// the same repository and would otherwise report every commit twice.
				seen := map[string]bool{}
				identityMissing := false
				for _, p := range repo.Paths {
					who := author
					if who == "" {
						// Per repository, because a work checkout and a personal one
						// can legitimately carry different identities.
						who = vcs.GitAuthorEmail(p.Path)
						if who == "" {
							// No explicit author and no resolvable git identity: querying
							// unfiltered would silently widen "what did I do" to everyone's
							// commits on this path (vcs.GitLog omits --author when empty).
							// Skip the path and say so, rather than guess wrong.
							identityMissing = true
							continue
						}
					}
					found, err := vcs.GitLog(p.Path, vcs.LogOptions{
						Since: since, Until: until, Author: who, Limit: maxCommitHits,
					})
					if err != nil {
						// A path that has been moved or deleted must not fail the
						// whole query — the other repositories still hold answers.
						if d.Logger != nil {
							d.Logger.Warn("list_commits: skipping unreadable repo path", zap.String("repo", full))
						}
						continue
					}
					scanned++
					for _, c := range found {
						if seen[c.Hash] || len(commits) >= maxCommitHits {
							continue
						}
						seen[c.Hash] = true
						commits = append(commits, map[string]any{
							"repo":       full,
							"hash":       c.Hash,
							"author":     c.Author,
							"email":      c.Email,
							"authorDate": c.AuthorDate,
							"commitDate": c.CommitDate,
							"subject":    c.Subject,
						})
					}
				}
				if identityMissing {
					noIdentity = append(noIdentity, full)
				}
			}

			return map[string]any{
				"count":                   len(commits),
				"commits":                 commits,
				"scannedPaths":            scanned,
				"reposWithoutLocalPath":   noLocal,
				"reposWithoutGitIdentity": noIdentity,
			}, nil
		},
	})
}
