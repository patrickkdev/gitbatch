# gitbatch

**Run common Git commands across many repositories — safely and simply.**

`gitbatch` is a lightweight Go CLI designed to execute `git status`, `git diff --no-pager`, `git pull`, `git add`, `git commit`, and `git push` across multiple repositories using glob patterns.

I built this tool because, while working with clients who had multiple websites with similar structures, I constantly found myself repeating the same Git commands across many clones. Often, I needed to make a change that affected all these repositories. `gitbatch` helps automate these repetitive tasks while keeping them safe and predictable.

---

## Key Features

* **Safe:** Avoid running commands in non-repositories. Destructive operations like `push --force` require confirmation.
* **Simple:** Small, predictable CLI with explicit flags.
* **Practical:** Supports recursive globbing (`**`) and streams output so you can debug per-repo issues.

---

## Installation

With Go installed, run:

```bash
# Install the latest version
go install github.com/patrickkdev/gitbatch@latest
```

This places the `gitbatch` binary in `$GOPATH/bin` or `$GOBIN`.

---

## Philosophy & Design Decisions

* **Reliable repository detection:** Uses `git rev-parse --is-inside-work-tree` instead of just checking for a `.git` folder. This works better with Git worktrees and nested setups.
* **Interactive confirmation for dangerous commands:** Pushes prompt for confirmation by default to prevent mass accidents.
* **Globbing with doublestar:** Enables recursive patterns like `projects/**/microservice-*` across platforms.
* **Built with Cobra:** Subcommands, flags, and help messages follow familiar patterns, making the CLI intuitive and easy to extend.
* **Streamed output & timeouts:** See logs/errors per repository immediately. Commands have sane timeouts to prevent hangs.
* **Sequential execution by default:** Safe and predictable; concurrency can be added later with a `--concurrency` flag.

---

## Commands

All commands accept one or more path patterns (globs). Only directories detected as Git repositories are processed.

### `gitbatch status <patterns...>`

Runs `git status` in each repository.

**Why:** Quickly check the state of multiple working trees (uncommitted changes, untracked files, current branches) before pulling or committing.

---

### `gitbatch diff <patterns...>`

Runs `git --no-pager diff` in each repository.

**Why:** Inspect differences across repositories without opening an editor. Useful for validating changes before committing.

---

### `gitbatch pull <patterns...>`

Runs `git pull` in each repository.

**Why:** Automates fetching and merging from remotes across multiple clones. It respects each repo’s configured merge strategy and remote.

---

### `gitbatch add -p <pathspec> <patterns...>`

Runs `git add -- <pathspec>` in each repository. Defaults to `.` if no pathspec is provided.

**Why:** Encourages safe, targeted additions. Using `--` and explicit pathspecs prevents accidentally adding unrelated files.

**Tip:** Use explicit pathspecs for large projects or nested structures to avoid unintended changes.

---

### `gitbatch commit -m "message" <patterns...>`

Runs `git commit -m "message"` in each repository. Skips repos with nothing to commit.

**Why:** Batch commits with a consistent message across multiple repos. Avoids interactive commit prompts, keeping automation-friendly behavior.

---

### `gitbatch push [--force] <patterns...>`

Runs `git push` in each repository.

* Prompts for confirmation by default.
* Use `--yes` to skip confirmation.
* Use `--force` with caution.

**Why:** Pushing changes is impactful. Confirmation helps prevent accidental mass updates to remotes.

---

## Examples

```bash
# Show status for multiple repositories
gitbatch status ./projects/*

# See diffs recursively
gitbatch diff "projects/**/service-*"

# Add JS files across repos
gitbatch add -p "src/**/*.js" repos/*

# Commit with a message
gitbatch commit -m "chore: bump deps" repos/*

# Push with confirmation
gitbatch push repos/*

# Skip push confirmation
gitbatch push --yes repos/*

# Force push (use with caution!)
gitbatch push --yes --force repos/*
```

> Tip: Quote glob patterns to let `gitbatch` handle expansion, especially on platforms where your shell might already expand globs.

---

## Internals / Implementation Notes

* CLI built with **Cobra** for commands and flags.
* Uses **doublestar** for recursive glob support.
* Repository detection via `git rev-parse --is-inside-work-tree`.
* Commands stream stdout/stderr per repository.
* Each Git invocation runs with a default timeout to avoid hangs.

---

## Safety & Best Practices

* **Review output** before committing/pushing. You’re still responsible for changes.
* **Use `--yes` only in trusted automation contexts**.
* **Avoid `--force` unless necessary**. Force-pushing rewrites history and can affect collaborators.

---

## Future Ideas

* `--concurrency` to run commands in parallel with per-repo timeouts.
* `--dry-run` to preview actions without modifying repositories.
* Interactive per-repo filtering or push confirmation.

---

## Contributing

Bug reports, PRs, and improvements are welcome. Please follow Go conventions, add tests for new behavior, and keep changes focused.

---

## License

This project is licensed under the **Apache License 2.0**.  
See the [LICENSE](./LICENSE) file for details.
