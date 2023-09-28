package gh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v47/github"
	"github.com/whywaita/myshoes/pkg/logger"
)

// ExistGitHubRunner check exist registered of GitHub runner
func ExistGitHubRunner(ctx context.Context, client *github.Client, owner, repo, runnerName string) (*github.Runner, error) {
	runners, err := ListRunners(ctx, client, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of runners: %w", err)
	}

	return ExistGitHubRunnerWithRunner(runners, runnerName)
}

// ExistGitHubRunnerWithRunner check exist registered of GitHub runner from a list of runner
func ExistGitHubRunnerWithRunner(runners []*github.Runner, runnerName string) (*github.Runner, error) {
	for _, r := range runners {
		if strings.EqualFold(r.GetName(), runnerName) {
			return r, nil
		}
	}

	return nil, ErrNotFound
}

// ListRunners get runners that registered repository or org
func ListRunners(ctx context.Context, client *github.Client, owner, repo string) ([]*github.Runner, error) {
	if cachedRs, found := responseCache.Get(getCacheKey(owner, repo)); found {
		logger.Logf(true, "Used cached list of runners for %s/%s", owner, repo)
		return cachedRs.([]*github.Runner), nil
	}

	var opts = &github.ListOptions{
		Page:    0,
		PerPage: 100,
	}

	var rs []*github.Runner
	for {
		logger.Logf(true, "get runners for %s/%s from GitHub, page: %d, now all runners: %d", owner, repo, opts.Page, len(rs))
		runners, resp, err := listRunners(ctx, client, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list runners: %w", err)
		}
		storeRateLimit(getRateLimitKey(owner, repo), resp.Rate)

		rs = append(rs, runners.Runners...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	responseCache.Set(getCacheKey(owner, repo), rs, 1*time.Second)
	logger.Logf(true, "found %d runners for %s/%s in GitHub", len(rs), owner, repo)

	return rs, nil
}

func getCacheKey(owner, repo string) string {
	return fmt.Sprintf("owner-%s-repo-%s", owner, repo)
}

func listRunners(ctx context.Context, client *github.Client, owner, repo string, opts *github.ListOptions) (*github.Runners, *github.Response, error) {
	if repo == "" {
		runners, resp, err := client.Actions.ListOrganizationRunners(ctx, owner, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list organization runners: %w", err)
		}
		return runners, resp, nil
	}

	runners, resp, err := client.Actions.ListRunners(ctx, owner, repo, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list repository runners: %w", err)
	}
	return runners, resp, nil
}

// GetLatestRunnerVersion get a latest version of actions/runner
func GetLatestRunnerVersion(ctx context.Context) (string, error) {
	client := github.NewClient(runnerVersionCacheTransport.Client())
	release, _, err := client.Repositories.GetLatestRelease(ctx, "actions", "runner")
	if err != nil {
		return "", fmt.Errorf("failed to get latest runner version: %w", err)
	}
	return *release.TagName, nil
}
