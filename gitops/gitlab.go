package gitops

import (
	"context"
	"fmt"

	"github.com/xanzy/go-gitlab"
)

// GitLabClient wraps the go-gitlab client and implements GitProvider.
type GitLabClient struct {
	client *gitlab.Client
}

// NewGitLabClient creates a GitLabClient authenticated with the given token.
func NewGitLabClient(token string) *GitLabClient {
	client, _ := gitlab.NewClient(token)
	return &GitLabClient{client: client}
}

// projectPath returns the "owner/repo" string that go-gitlab expects as a
// project identifier.
func projectPath(owner, repo string) string {
	return owner + "/" + repo
}

// CommitFiles creates an atomic commit with multiple file creates/updates
// using the GitLab Commits API.
func (g *GitLabClient) CommitFiles(ctx context.Context, owner, repo, branch, basePath string, files map[string]string, message string) (string, error) {
	pid := projectPath(owner, repo)

	// Ensure the branch exists. If not, create it from the default branch.
	_, _, err := g.client.Branches.GetBranch(pid, branch, gitlab.WithContext(ctx))
	if err != nil {
		_, _, createErr := g.client.Branches.CreateBranch(pid, &gitlab.CreateBranchOptions{
			Branch: gitlab.Ptr(branch),
			Ref:    gitlab.Ptr("main"),
		}, gitlab.WithContext(ctx))
		if createErr != nil {
			return "", fmt.Errorf("creating branch %s: %w", branch, createErr)
		}
	}

	// Build commit actions for each file.
	var actions []*gitlab.CommitActionOptions
	for path, content := range files {
		fullPath := basePath + "/" + path
		action := determineAction(ctx, g.client, pid, branch, fullPath)
		actions = append(actions, &gitlab.CommitActionOptions{
			Action:   gitlab.Ptr(action),
			FilePath: gitlab.Ptr(fullPath),
			Content:  gitlab.Ptr(content),
		})
	}

	commit, _, err := g.client.Commits.CreateCommit(pid, &gitlab.CreateCommitOptions{
		Branch:        gitlab.Ptr(branch),
		CommitMessage: gitlab.Ptr(message),
		Actions:       actions,
	}, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}

	return commit.ID, nil
}

// determineAction checks whether a file exists to decide between create and update.
func determineAction(ctx context.Context, client *gitlab.Client, pid, branch, path string) gitlab.FileActionValue {
	_, _, err := client.RepositoryFiles.GetFile(pid, path, &gitlab.GetFileOptions{
		Ref: gitlab.Ptr(branch),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return gitlab.FileCreate
	}
	return gitlab.FileUpdate
}

// CreatePR opens a merge request from head into base on the given project.
func (g *GitLabClient) CreatePR(ctx context.Context, owner, repo, head, base, title, body string) (string, error) {
	pid := projectPath(owner, repo)

	mr, _, err := g.client.MergeRequests.CreateMergeRequest(pid, &gitlab.CreateMergeRequestOptions{
		Title:        gitlab.Ptr(title),
		Description:  gitlab.Ptr(body),
		SourceBranch: gitlab.Ptr(head),
		TargetBranch: gitlab.Ptr(base),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("creating merge request: %w", err)
	}

	return mr.WebURL, nil
}

// GetFileContent retrieves the content of a single file from the repository.
func (g *GitLabClient) GetFileContent(ctx context.Context, owner, repo, branch, path string) (string, error) {
	pid := projectPath(owner, repo)

	file, _, err := g.client.RepositoryFiles.GetFile(pid, path, &gitlab.GetFileOptions{
		Ref: gitlab.Ptr(branch),
	}, gitlab.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("getting file %s: %w", path, err)
	}

	return file.Content, nil
}
