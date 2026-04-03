package gitops

import (
	"context"
	"fmt"

	"github.com/google/go-github/v68/github"
)

// GitHubClient wraps the go-github client and implements GitProvider.
type GitHubClient struct {
	client *github.Client
}

// NewGitHubClient creates a GitHubClient authenticated with the given token.
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		client: github.NewClient(nil).WithAuthToken(token),
	}
}

// CommitFiles creates an atomic commit that creates or updates multiple files
// using the low-level Git tree/blob/commit API.
func (g *GitHubClient) CommitFiles(ctx context.Context, owner, repo, branch, basePath string, files map[string]string, message string) (string, error) {
	// Get the reference for the target branch.
	ref, _, err := g.client.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
	if err != nil {
		// Branch may not exist yet; create it from the default branch HEAD.
		defaultRef, _, defErr := g.client.Git.GetRef(ctx, owner, repo, "refs/heads/main")
		if defErr != nil {
			return "", fmt.Errorf("getting default branch ref: %w", defErr)
		}
		newRef := &github.Reference{
			Ref:    github.Ptr("refs/heads/" + branch),
			Object: defaultRef.Object,
		}
		ref, _, err = g.client.Git.CreateRef(ctx, owner, repo, newRef)
		if err != nil {
			return "", fmt.Errorf("creating branch %s: %w", branch, err)
		}
	}

	baseCommitSHA := ref.GetObject().GetSHA()

	// Get the tree of the base commit.
	baseCommit, _, err := g.client.Git.GetCommit(ctx, owner, repo, baseCommitSHA)
	if err != nil {
		return "", fmt.Errorf("getting base commit: %w", err)
	}
	baseTreeSHA := baseCommit.GetTree().GetSHA()

	// Build tree entries for each file.
	var entries []*github.TreeEntry
	for path, content := range files {
		fullPath := basePath + "/" + path
		entries = append(entries, &github.TreeEntry{
			Path:    github.Ptr(fullPath),
			Mode:    github.Ptr("100644"),
			Type:    github.Ptr("blob"),
			Content: github.Ptr(content),
		})
	}

	// Create a new tree.
	newTree, _, err := g.client.Git.CreateTree(ctx, owner, repo, baseTreeSHA, entries)
	if err != nil {
		return "", fmt.Errorf("creating tree: %w", err)
	}

	// Create the commit.
	newCommit, _, err := g.client.Git.CreateCommit(ctx, owner, repo, &github.Commit{
		Message: github.Ptr(message),
		Tree:    newTree,
		Parents: []*github.Commit{{SHA: github.Ptr(baseCommitSHA)}},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}

	// Update the branch reference to point to the new commit.
	ref.Object.SHA = newCommit.SHA
	_, _, err = g.client.Git.UpdateRef(ctx, owner, repo, ref, false)
	if err != nil {
		return "", fmt.Errorf("updating ref: %w", err)
	}

	return newCommit.GetSHA(), nil
}

// CreatePR opens a pull request from head into base on the given repository.
func (g *GitHubClient) CreatePR(ctx context.Context, owner, repo, head, base, title, body string) (string, error) {
	pr, _, err := g.client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(body),
		Head:  github.Ptr(head),
		Base:  github.Ptr(base),
	})
	if err != nil {
		return "", fmt.Errorf("creating pull request: %w", err)
	}

	return pr.GetHTMLURL(), nil
}

// GetFileContent retrieves the content of a single file from the repository.
func (g *GitHubClient) GetFileContent(ctx context.Context, owner, repo, branch, path string) (string, error) {
	opts := &github.RepositoryContentGetOptions{Ref: branch}
	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", fmt.Errorf("getting file %s: %w", path, err)
	}
	if fileContent == nil {
		return "", fmt.Errorf("path %s is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("decoding file content: %w", err)
	}

	return content, nil
}
