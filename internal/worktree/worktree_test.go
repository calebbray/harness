package worktree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(t.TempDir(), "task-a")

	wt, err := Add(repo, path, "task/a", "master")
	require.NoError(t, err)
	assert.Equal(t, path, wt.Path)
	assert.Equal(t, "task/a", wt.Branch)

	_, err = os.Stat(filepath.Join(path, "README.md"))
	assert.NoError(t, err)
}

func TestCommit(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(t.TempDir(), "task-a")
	wt, err := Add(repo, path, "task/a", "master")
	require.NoError(t, err)

	t.Run("nothing to commit", func(t *testing.T) {
		committed, err := CommitWorktree(wt, "no changes")
		assert.NoError(t, err)
		assert.False(t, committed)
	})

	t.Run("commits a real change onto the task branch not master", func(t *testing.T) {
		require.NoError(t, os.WriteFile(filepath.Join(wt.Path, "new.txt"), []byte("hello\n"), 0o644))

		committed, err := CommitWorktree(wt, "add new.txt")
		require.NoError(t, err)
		require.True(t, committed)

		masterLog, err := run(repo, "log", "master", "--oneline")
		require.NoError(t, err)
		require.NotContains(t, masterLog, "add new.txt")

		branchLog, err := run(wt.Path, "log", "--oneline")
		require.NoError(t, err)
		require.Contains(t, branchLog, "add new.txt")
	})
}

func TestChainedWorktree(t *testing.T) {
	repo := initRepo(t)

	a, err := Add(repo, filepath.Join(t.TempDir(), "task-a"), "task/a", "master")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(a.Path, "a.txt"), []byte("from a\n"), 0o644))
	committed, err := CommitWorktree(a, "task a work")
	require.NoError(t, err)
	require.True(t, committed)

	b, err := Add(repo, filepath.Join(t.TempDir(), "task-b"), "task/b", "task/a")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(b.Path, "a.txt"))
	require.NoError(t, err, "task/b's worktree should already contain task/a's committed work")
}

func TestRemoveAndDeleteBranch(t *testing.T) {
	repo := initRepo(t)
	path := filepath.Join(t.TempDir(), "task-a")
	wt, err := Add(repo, path, "task/a", "master")
	require.NoError(t, err)

	require.NoError(t, Remove(repo, wt))
	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err))

	require.NoError(t, DeleteBranch(repo, "task/a"))
}

func initRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()

	_, err := run(dir, "init", "-b", "master")
	require.NoError(t, err)

	_, err = run(dir, "config", "user.email", "test@example.com")
	require.NoError(t, err)
	_, err = run(dir, "config", "user.name", "Test")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("initial\n"), 0o644))

	_, err = run(dir, "add", "-A")
	require.NoError(t, err)

	_, err = run(dir, "commit", "-m", "initial commit")
	require.NoError(t, err)

	return dir
}
