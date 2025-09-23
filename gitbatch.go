// gitbatch — simple, safe CLI to run git commands across multiple repositories.
//
// Build: go build -o gitbatch
// Usage examples:
//   ./gitbatch status ./projects/*
//   ./gitbatch diff "repos/**"
//   ./gitbatch pull repos/*
//   ./gitbatch add -p "src/*.js" repos/*
//   ./gitbatch commit -m "Fix typo" repos/*
//   ./gitbatch push repos/*     # asks for confirmation
//   ./gitbatch push --yes repos/*  # skip confirmation

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/spf13/cobra"
)

const defaultTimeout = 2 * time.Minute

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "gitbatch [command] <path-pattern>...",
	Short: "Run common git commands across many repos (supports globs)",
	Long: `gitbatch finds directories that contain git repositories and runs
specified git commands inside each repo. Patterns support shell globs and
recursive ** patterns (doublestar).`,
	Args: cobra.MinimumNArgs(1),
}

func collectRepos(patterns []string) ([]string, error) {
	seen := map[string]struct{}{}
	var repos []string
	for _, pat := range patterns {
		matches, err := doublestar.Glob(os.DirFS("."), pat)
		if err != nil {
			// try fallback to filepath.Glob (handles simple globs and cases where shell already expanded)
			matches2, err2 := filepath.Glob(pat)
			if err2 != nil {
				return nil, fmt.Errorf("invalid pattern %q: %v", pat, err)
			}
			matches = matches2
		}

		// doublestar.Glob returns paths relative to FS root; convert to OS paths
		for _, m := range matches {
			// doublestar returns paths with unix separators when using DirFS; ensure correct OS path
			mp := filepath.FromSlash(m)
			abs, err := filepath.Abs(mp)
			if err != nil {
				abs = mp
			}
			fi, err := os.Stat(abs)
			if err != nil {
				continue
			}
			if !fi.IsDir() {
				// if it's a file, consider its parent
				abs = filepath.Dir(abs)
			}
			if _, ok := seen[abs]; ok {
				continue
			}
			if isGitRepo(abs) {
				seen[abs] = struct{}{}
				repos = append(repos, abs)
			}
		}
	}
	if len(repos) == 0 {
		return nil, errors.New("no git repositories found for given pattern(s)")
	}
	return repos, nil
}

func isGitRepo(dir string) bool {
	// Prefer calling git to detect repository (handles git worktrees and submodules)
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runGitCapture(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	return string(b), err
}

// status command
var statusCmd = &cobra.Command{
	Use:   "status <pattern>...",
	Short: "Run git status in matching repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			if err := runGit(ctx, r, "status"); err != nil {
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

// diff command
var diffCmd = &cobra.Command{
	Use:   "diff <pattern>...",
	Short: "Run git --no-pager diff in matching repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			if err := runGit(ctx, r, "--no-pager", "diff"); err != nil {
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

// pull command
var pullCmd = &cobra.Command{
	Use:   "pull <pattern>...",
	Short: "Run git pull in matching repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			if err := runGit(ctx, r, "pull"); err != nil {
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

// add command
var addPathSpec string
var addCmd = &cobra.Command{
	Use:   "add [--pathspec <path>] <pattern>...",
	Short: "Run git add (safe) in matching repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		if addPathSpec == "" {
			addPathSpec = "."
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			if err := runGit(ctx, r, "add", "--", addPathSpec); err != nil {
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

// commit command
var commitMsg string
var commitCmd = &cobra.Command{
	Use:   "commit -m <message> <pattern>...",
	Short: "Run git commit with the provided message in matching repositories",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(commitMsg) == "" {
			return errors.New("commit message required: use -m \"message\"")
		}
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			// Use -m, but allow git to skip if there's nothing to commit
			out, err := runGitCapture(ctx, r, "commit", "-m", commitMsg)
			fmt.Print(out)
			if err != nil {
				// if exit status is 1 and message indicates nothing to commit, ignore
				if strings.Contains(out, "nothing to commit") || strings.Contains(out, "nothing added to commit") {
					continue
				}
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

// push command
var pushForce bool
var pushYes bool
var pushCmd = &cobra.Command{
	Use:   "push <pattern>...",
	Short: "Run git push in matching repositories (asks confirmation)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := collectRepos(args)
		if err != nil {
			return err
		}
		if !pushYes {
			fmt.Printf("About to push to %d repositories. This will contact remotes and may change remote history. Continue? (y/N): ", len(repos))
			s := userConfirm()
			if !s {
				fmt.Println("aborted")
				return nil
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		for _, r := range repos {
			fmt.Printf("\n---- %s ----\n", r)
			args := []string{"push"}
			if pushForce {
				args = append(args, "--force")
			}
			if err := runGit(ctx, r, args...); err != nil {
				fmt.Fprintf(os.Stderr, "error in %s: %v\n", r, err)
			}
		}
		return nil
	},
}

func userConfirm() bool {
	s := bufio.NewScanner(os.Stdin)
	if s.Scan() {
		text := strings.TrimSpace(strings.ToLower(s.Text()))
		return text == "y" || text == "yes"
	}
	return false
}

func init() {
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(pushCmd)

	addCmd.Flags().StringVarP(&addPathSpec, "pathspec", "p", ".", "pathspec to add (defaults to '.')")

	commitCmd.Flags().StringVarP(&commitMsg, "message", "m", "", "commit message (required)")
	commitCmd.MarkFlagRequired("message")

	pushCmd.Flags().BoolVarP(&pushForce, "force", "f", false, "force push (use with caution)")
	pushCmd.Flags().BoolVarP(&pushYes, "yes", "y", false, "skip confirmation for push")
}
