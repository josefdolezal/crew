// Package gitx wraps the git worktree operations crew needs: the CLI
// creates a worktree per agent at spawn, the daemon removes it at kill
// when it holds no work.
package gitx

import (
	"fmt"
	"os/exec"
	"strings"
)

func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// Toplevel returns the working-tree root containing dir, or an error if
// dir is not inside a git repository.
func Toplevel(dir string) (string, error) {
	return git(dir, "rev-parse", "--show-toplevel")
}

// AddWorktree creates a worktree at path with a new branch, based on the
// repository containing repoDir.
func AddWorktree(repoDir, path, branch string) error {
	if _, err := git(repoDir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("%s is not inside a git repository (required for --worktree)", repoDir)
	}
	_, err := git(repoDir, "worktree", "add", "-b", branch, path)
	return err
}

// RemoveResult says what happened to an agent's worktree at kill time.
type RemoveResult string

const (
	Removed RemoveResult = "removed"
	Kept    RemoveResult = "kept" // uncommitted changes or unmerged commits
)

// RemoveWorktreeIfClean removes the worktree (and its branch, if fully
// merged) unless it has uncommitted changes. Mirrors Claude Code's
// worktree isolation: clean worktrees vanish, work-in-progress survives.
func RemoveWorktreeIfClean(path string) (RemoveResult, error) {
	status, err := git(path, "status", "--porcelain")
	if err != nil {
		return Kept, err
	}
	if status != "" {
		return Kept, nil
	}
	branch, _ := git(path, "rev-parse", "--abbrev-ref", "HEAD")
	// Resolve the main repository so git isn't asked to remove the
	// worktree it is currently operating in.
	commonDir, err := git(path, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Kept, err
	}
	repoRoot := strings.TrimSuffix(commonDir, "/.git")
	if _, err := git(repoRoot, "worktree", "remove", path); err != nil {
		return Kept, err
	}
	if branch != "" && branch != "HEAD" {
		// -d only deletes fully merged branches; a branch holding real
		// commits survives on purpose.
		_, _ = git(repoRoot, "branch", "-d", branch)
	}
	return Removed, nil
}
