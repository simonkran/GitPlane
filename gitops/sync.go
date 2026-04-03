// Package gitops provides Git-based deployment integration for GitPlane,
// supporting atomic manifest commits and pull request workflows.
package gitops

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// GitProvider defines the interface that any Git hosting backend must implement.
type GitProvider interface {
	// CommitFiles atomically creates or updates files in a repository.
	CommitFiles(ctx context.Context, owner, repo, branch, basePath string, files map[string]string, message string) (commitSHA string, err error)
	// CreatePR opens a pull/merge request from head into base.
	CreatePR(ctx context.Context, owner, repo, head, base, title, body string) (prURL string, err error)
	// GetFileContent returns the content of a single file at the given branch.
	GetFileContent(ctx context.Context, owner, repo, branch, path string) (string, error)
}

// SyncService orchestrates committing generated manifests to a Git repository,
// either directly or via pull request.
type SyncService struct {
	provider      GitProvider
	owner         string
	repo          string
	defaultBranch string
}

// NewSyncService creates a SyncService by parsing the repository URL and
// storing the provider and default branch.
func NewSyncService(provider GitProvider, repoURL, defaultBranch string) (*SyncService, error) {
	owner, repo, err := parseOwnerRepo(repoURL)
	if err != nil {
		return nil, fmt.Errorf("parsing repo URL: %w", err)
	}

	return &SyncService{
		provider:      provider,
		owner:         owner,
		repo:          repo,
		defaultBranch: defaultBranch,
	}, nil
}

// CommitManifests commits the given files directly to the default branch under
// the specified git path prefix.
func (s *SyncService) CommitManifests(ctx context.Context, files map[string]string, clusterName, gitPath, message string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files to commit")
	}

	if message == "" {
		message = fmt.Sprintf("gitplane: update manifests for cluster %s", clusterName)
	}

	commitSHA, err := s.provider.CommitFiles(ctx, s.owner, s.repo, s.defaultBranch, gitPath, files, message)
	if err != nil {
		return "", fmt.Errorf("committing manifests for cluster %s: %w", clusterName, err)
	}

	return commitSHA, nil
}

// CommitManifestsWithPR creates a feature branch, commits the files, and opens
// a pull request against the default branch. This is the preferred workflow
// for production-stage clusters.
func (s *SyncService) CommitManifestsWithPR(ctx context.Context, files map[string]string, clusterName, gitPath, message string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("no files to commit")
	}

	branchName := fmt.Sprintf("gitplane/%s/%d", clusterName, time.Now().Unix())

	if message == "" {
		message = fmt.Sprintf("gitplane: update manifests for cluster %s", clusterName)
	}

	_, err := s.provider.CommitFiles(ctx, s.owner, s.repo, branchName, gitPath, files, message)
	if err != nil {
		return "", fmt.Errorf("committing to branch %s: %w", branchName, err)
	}

	title := fmt.Sprintf("GitPlane: update %s manifests", clusterName)
	body := fmt.Sprintf("Automated manifest update for cluster **%s**.\n\nPath: `%s`\n\n%s", clusterName, gitPath, message)

	prURL, err := s.provider.CreatePR(ctx, s.owner, s.repo, branchName, s.defaultBranch, title, body)
	if err != nil {
		return "", fmt.Errorf("creating PR for cluster %s: %w", clusterName, err)
	}

	return prURL, nil
}

// parseOwnerRepo extracts the owner and repository name from a Git URL.
// Supports HTTPS URLs (https://github.com/owner/repo.git) and
// SSH URLs (git@github.com:owner/repo.git).
func parseOwnerRepo(repoURL string) (owner, repo string, err error) {
	// Handle SSH-style URLs: git@host:owner/repo.git
	if strings.Contains(repoURL, ":") && strings.HasPrefix(repoURL, "git@") {
		parts := strings.SplitN(repoURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH URL: %s", repoURL)
		}
		return splitOwnerRepo(parts[1])
	}

	// Handle HTTPS URLs.
	parsed, err := url.Parse(repoURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL %q: %w", repoURL, err)
	}

	return splitOwnerRepo(strings.TrimPrefix(parsed.Path, "/"))
}

func splitOwnerRepo(path string) (string, string, error) {
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from path %q", path)
	}

	return parts[0], parts[1], nil
}
