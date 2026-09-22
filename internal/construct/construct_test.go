package construct

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/orchestrator"
	"github.com/calebbray/personal-agent/internal/plan"
	"github.com/calebbray/personal-agent/internal/worktree"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteChainOfThree(t *testing.T) {
	repo := initRepo(t)
	root := t.TempDir()

	a, err := plan.NewTaskNode("a", "first")
	require.NoError(t, err)
	b, err := plan.NewTaskNode("b", "second", a.Id)
	require.NoError(t, err)
	c, err := plan.NewTaskNode("c", "third", b.Id)
	require.NoError(t, err)

	p, err := plan.NewPlan("chain", a, b, c)
	require.NoError(t, err)

	cfg := Config{
		RepoDir:      repo,
		WorktreeRoot: root,
		BaseRef:      "master",
		Work: func(worktreePath string, n *plan.TaskNode) error {
			return os.WriteFile(filepath.Join(worktreePath, n.Title+".marker"), []byte("done"), 0o644)
		},
	}

	require.NoError(t, orchestrator.Run(p, Execute(cfg)))
	for _, n := range p.Nodes {
		assert.Equal(t, plan.Approved, n.Status)
	}

	requireMarkers(t, root, c.Id, "a.marker", "b.marker", "c.marker")
	requireMarkers(t, root, b.Id, "a.marker", "b.marker")
	requireMarkers(t, root, a.Id, "a.marker")

	requireBranchExists(t, repo, "task/"+a.Id.String())
	requireBranchExists(t, repo, "task/"+b.Id.String())
	requireBranchExists(t, repo, "task/"+c.Id.String())
}

func TestExecuteMultiParentGuard(t *testing.T) {
	repo := initRepo(t)
	root := t.TempDir()

	a, err := plan.NewTaskNode("a", "first")
	require.NoError(t, err)
	b, err := plan.NewTaskNode("b", "second")
	require.NoError(t, err)
	c, err := plan.NewTaskNode("c", "third", a.Id, b.Id) // two deps

	cfg := Config{RepoDir: repo, WorktreeRoot: root, BaseRef: "master", Work: noopWork}

	err = Execute(cfg)(c)
	require.Error(t, err)
}

func TestExecuteFailurePath(t *testing.T) {
	repo := initRepo(t)
	root := t.TempDir()

	a, err := plan.NewTaskNode("a", "first")
	require.NoError(t, err)
	b, err := plan.NewTaskNode("b", "second", a.Id)
	require.NoError(t, err)

	p, err := plan.NewPlan("chain", a, b)
	require.NoError(t, err)

	cfg := Config{
		RepoDir:      repo,
		WorktreeRoot: root,
		BaseRef:      "master",
		Work: func(worktreePath string, n *plan.TaskNode) error {
			if n.Id == a.Id {
				return errors.New("boom")
			}
			return nil
		},
	}

	err = orchestrator.Run(p, Execute(cfg))
	require.Error(t, err) // plan gets stuck: b can never become ready

	assert.Equal(t, plan.Failed, a.Status)
	assert.Contains(t, p.Blocked(), b)

	// the "no cleanup" assertion — a's worktree should still exist on disk
	_, statErr := os.Stat(filepath.Join(root, a.Id.String()))
	assert.NoError(t, statErr)
}

func TestAgentWorkWritesFilesInsideWorktree(t *testing.T) {
	worktreeDir := t.TempDir()

	n, err := plan.NewTaskNode("write a greeting", "create hello.txt with a friendly message")
	require.NoError(t, err)

	writeInput, err := json.Marshal(map[string]string{
		"path":     "hello.txt",
		"contents": "hi from the worker",
	})
	require.NoError(t, err)

	mock := &client.MockClient{
		Responses: []*client.Response{
			{
				StopReason: client.StopReasonToolUse,
				Content: []client.Content{
					client.ToolUseContent{ContentType: "tool_use", Id: "call_1", Name: "write_file", Input: writeInput},
				},
			},
			{
				StopReason: client.StopReasonEndTurn,
				Content:    []client.Content{client.TextContent{ContentType: "text", Text: "wrote hello.txt"}},
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err = AgentWork(mock, logger)(worktreeDir, n)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(worktreeDir, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hi from the worker", string(data))
}

func requireMarkers(t *testing.T, root string, id uuid.UUID, filenames ...string) {
	t.Helper()
	for _, file := range filenames {
		_, err := os.Stat(filepath.Join(root, id.String(), file))
		require.NoError(t, err)
	}
}

func requireBranchExists(t *testing.T, dir, branch string) {
	t.Helper()
	exists, err := worktree.ListBranch(dir, branch)
	require.NoError(t, err)
	require.True(t, exists, "expected branch %q to exist", branch)
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	require.NoError(t, worktree.Init(dir))
	require.NoError(t, worktree.Config(dir, "user.email", "test@example.com"))
	require.NoError(t, worktree.Config(dir, "user.name", "Test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("initial\n"), 0o644))
	require.NoError(t, worktree.Stage(dir, "-A"))
	require.NoError(t, worktree.Commit(dir, "initial commit"))
	return dir
}

func noopWork(string, *plan.TaskNode) error {
	return nil
}
