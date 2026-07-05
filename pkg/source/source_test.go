package source

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var logger = slog.New(slog.DiscardHandler)

func TestIsGitURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		arg  string
		want bool
	}{
		{"https://github.com/longhorn/website", true},
		{"http://example.com/site", true},
		{"git@github.com:longhorn/website.git", true},
		{"ssh://git@github.com/longhorn/website", true},
		{"file:///srv/repos/website", true},
		{"/srv/repos/website.git", true},
		{"./website", false},
		{"../sites/longhorn", false},
		{"/home/user/website", false},
		{"website", false},
	}
	for _, tt := range tests {
		if got := IsGitURL(tt.arg); got != tt.want {
			t.Errorf("IsGitURL(%q) = %v, want %v", tt.arg, got, tt.want)
		}
	}
}

func TestResolveLocalPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, cleanup, err := Resolve(t.Context(), dir, logger)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	defer cleanup()
	if got != dir {
		t.Errorf("Resolve returned %q, want %q", got, dir)
	}
}

func TestResolveClone(t *testing.T) {
	t.Parallel()

	// Create a fixture repo with one committed file; clone it via file://.
	repoDir := t.TempDir()
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatalf("PlainInit failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config.toml"), []byte("title = \"Fixture\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("Worktree failed: %v", err)
	}
	if _, err := worktree.Add("config.toml"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	_, err = worktree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com"},
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	dir, cleanup, err := Resolve(t.Context(), "file://"+repoDir, logger)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err != nil {
		t.Errorf("cloned repo missing config.toml: %v", err)
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("cleanup did not remove clone directory %s", dir)
	}
}

func TestResolveErrors(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, arg string }{
		{"missing path", filepath.Join(t.TempDir(), "nope")},
		{"file instead of directory", file},
		{"clone failure", "file://" + filepath.Join(t.TempDir(), "no-repo")},
	}
	for _, tt := range tests {
		if _, _, err := Resolve(t.Context(), tt.arg, logger); err == nil {
			t.Errorf("%s: expected error, got nil", tt.name)
		}
	}
}
