package worktree

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Worktree struct {
	Path   string // absolute path on disk to the worktree dir on disk
	Branch string
}

func Add(repoDir, path, branch, baseRef string) (*Worktree, error) {
	if _, err := run(repoDir, "worktree", "add", "-b", branch, path, baseRef); err != nil {
		return nil, fmt.Errorf("creating worktree for branch %q from %q: %w", branch, baseRef, err)
	}
	return &Worktree{Path: path, Branch: branch}, nil
}

func Remove(repoDir string, w *Worktree) error {
	_, err := run(repoDir, "worktree", "remove", w.Path)
	return err
}

func DeleteBranch(repoDir, branch string) error {
	_, err := run(repoDir, "branch", "-d", branch)
	return err
}

// TODO: use the git functions instead of raw run calls
func CommitWorktree(w *Worktree, message string) (committed bool, err error) {
	if _, err := run(w.Path, "add", "-A"); err != nil {
		return false, fmt.Errorf("staging changes: %w", err)
	}

	// exits 0 if nothing is staged
	// exits 1 if something is staged
	//   - not the usual error means something went wrong
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	cmd.Dir = w.Path
	if err := cmd.Run(); err == nil {
		// nothing changed, nothing committed, not an error
		return false, nil
	}

	if _, err := run(w.Path, "commit", "-m", message); err != nil {
		return false, fmt.Errorf("committing: %w", err)
	}

	return true, nil
}

func Init(dir string) error {
	_, err := run(dir, "init", "-b", "master")
	return err
}

func Config(dir, field, value string) error {
	_, err := run(dir, "config", field, value)
	return err
}

func Stage(dir string, args ...string) error {
	args = append([]string{"add"}, args...)
	_, err := run(dir, args...)
	return err
}

func Commit(dir, message string) error {
	_, err := run(dir, "commit", "-m", message)
	return err
}

func ListBranch(dir, branchName string) (exists bool, err error) {
	out, err := run(dir, "branch", "--list", branchName)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}

	return out.String(), nil
}
