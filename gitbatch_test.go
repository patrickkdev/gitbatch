package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// initTestRepo creates a git repo in its own temp dir and returns the path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initTestRepoAt(t, dir)
	return dir
}

// initTestRepoAt initializes a git repo at the given path (creates the dir if needed)
// and sets minimal user config so commits work in tests.
func initTestRepoAt(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed for %s: %v, out=%s", dir, err, string(out))
	}

	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("failed to run git %v in %s: %v, out=%s", args, dir, err, string(out))
		}
	}
	// minimal config so commits do not fail
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "tester")
}

func TestIsGitRepo(t *testing.T) {
	repo := initTestRepo(t)
	if !isGitRepo(repo) {
		t.Errorf("expected %s to be a git repo", repo)
	}

	nonRepo := t.TempDir()
	if isGitRepo(nonRepo) {
		t.Errorf("expected %s to not be a git repo", nonRepo)
	}
}

func TestCollectRepos(t *testing.T) {
	// Create a workspace and put repos inside it so globbing against the CWD works reliably.
	workspace := t.TempDir()

	repo1 := filepath.Join(workspace, "repo1")
	initTestRepoAt(t, repo1)

	repo2 := filepath.Join(workspace, "repo2")
	initTestRepoAt(t, repo2)

	nested := filepath.Join(workspace, "nested")
	repo3 := filepath.Join(nested, "repo3")
	initTestRepoAt(t, repo3)

	// Change working dir to workspace for the duration of this test so collectRepos'
	// doublestar.Glob(os.DirFS("."), pattern) will match our relative patterns.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatalf("failed to chdir to workspace: %v", err)
	}
	// Restore working directory when test finishes.
	t.Cleanup(func() {
		_ = os.Chdir(origWD)
	})

	// Use patterns that doublestar will match relative to workspace (CWD).
	repos, err := collectRepos([]string{"repo*", "nested/**"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// We expect all three repos to be present (order is not important).
	found := map[string]bool{}
	for _, r := range repos {
		// convert to absolute to compare
		abs, err := filepath.Abs(r)
		if err != nil {
			abs = r
		}
		found[abs] = true
	}

	expectAbs := func(p string) string {
		a, err := filepath.Abs(p)
		if err != nil {
			return p
		}
		return a
	}

	if !found[expectAbs(repo1)] || !found[expectAbs(repo2)] || !found[expectAbs(repo3)] {
		t.Fatalf("collectRepos did not return expected repos; got=%v", repos)
	}
}

func TestRunGitAndCapture(t *testing.T) {
	repo := initTestRepo(t)

	// create a file and commit
	file := filepath.Join(repo, "test.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := runGit(ctx, repo, "add", "test.txt"); err != nil {
		t.Fatalf("git add failed: %v", err)
	}

	if out, err := runGitCapture(ctx, repo, "commit", "-m", "add test.txt"); err != nil {
		t.Fatalf("git commit failed: %v, out=%s", err, out)
	}

	out, err := runGitCapture(ctx, repo, "log", "--oneline")
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(out, "add test.txt") {
		t.Errorf("expected commit message in log, got %q", out)
	}
}

func TestUserConfirm(t *testing.T) {
	// override os.Stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// case: yes
	r1, w1, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r1
	_, _ = w1.Write([]byte("yes\n"))
	_ = w1.Close()
	if !userConfirm() {
		t.Errorf("expected userConfirm to return true for 'yes'")
	}

	// case: no
	r2, w2, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r2
	_, _ = w2.Write([]byte("n\n"))
	_ = w2.Close()
	if userConfirm() {
		t.Errorf("expected userConfirm to return false for 'n'")
	}
}

func TestCollectReposNoMatch(t *testing.T) {
	// run in an empty temp dir to make sure the pattern definitely doesn't match.
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	_, err = collectRepos([]string{"nonexistent/path/*"})
	if err == nil {
		t.Errorf("expected error when no repos found")
	}
}
