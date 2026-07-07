// Package source resolves a site source argument — a local directory or a git
// URL — into a local directory to read the Hugo site from.
package source

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
)

// IsGitURL reports whether arg looks like a git remote URL rather than a
// local path.
func IsGitURL(arg string) bool {
	return strings.HasPrefix(arg, "https://") ||
		strings.HasPrefix(arg, "http://") ||
		strings.HasPrefix(arg, "git@") ||
		strings.HasPrefix(arg, "ssh://") ||
		strings.HasPrefix(arg, "file://") ||
		strings.HasSuffix(arg, ".git")
}

// Resolve turns arg into a local directory containing the site source. Git
// URLs are shallow-cloned (default branch only) into a temporary directory;
// local paths are verified and passed through. The returned cleanup removes
// the temporary clone and is a no-op for local paths.
func Resolve(ctx context.Context, arg string, logger *slog.Logger) (dir string, cleanup func(), err error) {
	noop := func() {}

	if !IsGitURL(arg) {
		info, err := os.Stat(arg)
		if err != nil {
			return "", noop, fmt.Errorf("failed to stat site path %s: %w", arg, err)
		}
		if !info.IsDir() {
			return "", noop, fmt.Errorf("site path %s is not a directory", arg)
		}
		logger.DebugContext(ctx, "using local site directory", "dir", arg)
		return arg, noop, nil
	}

	tmpDir, err := os.MkdirTemp("", "hugo-to-skill-*")
	if err != nil {
		return "", noop, fmt.Errorf("failed to create temporary clone directory: %w", err)
	}
	cleanup = func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			logger.WarnContext(ctx, "failed to remove temporary clone", "dir", tmpDir, "error", err)
		}
	}

	// Clone into a subdirectory named after the repository so consumers that
	// fall back to the directory basename (e.g. the site title) see the repo
	// name, not the temp-dir naming scheme.
	cloneDir := filepath.Join(tmpDir, repoName(arg))
	logger.InfoContext(ctx, "cloning site", "url", arg, "dir", cloneDir)
	start := time.Now()
	_, err = git.PlainCloneContext(ctx, cloneDir, false, &git.CloneOptions{
		URL:          arg,
		Depth:        1,
		SingleBranch: true,
		Tags:         git.NoTags,
	})
	if err != nil {
		cleanup()
		return "", noop, fmt.Errorf("failed to clone %s: %w", arg, err)
	}
	logger.InfoContext(ctx, "clone complete", "duration", time.Since(start).Round(time.Millisecond))

	return cloneDir, cleanup, nil
}

// repoName derives the repository name from a git URL: the last path
// component without a .git suffix.
func repoName(arg string) string {
	name := strings.TrimRight(arg, "/")
	name = strings.TrimSuffix(name, ".git")
	if i := strings.LastIndexAny(name, "/:"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "site"
	}
	return name
}
