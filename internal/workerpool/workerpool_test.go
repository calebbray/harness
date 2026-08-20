package workerpool

//
// import (
// 	"io"
// 	"log/slog"
// 	"sync"
// 	"testing"
//
// 	"github.com/calebbray/personal-agent/internal/client"
// 	"github.com/calebbray/personal-agent/internal/tools"
// 	"github.com/calebbray/personal-agent/internal/tools/jira"
// 	"github.com/stretchr/testify/require"
// )
//
// func TestWorkerPoolRace(t *testing.T) {
// 	c, err := client.New("mock")
// 	require.NoError(t, err)
// 	cfg := TaskConfig{
// 		Client: c,
// 		Tools:  tools.Default(MockSaver{}),
// 		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
// 	}
//
// 	wp := New(3, cfg)
// 	wp.Run()
//
// 	const numJobs = 50
// 	jobs := make([]*Job, numJobs)
//
// 	var readers sync.WaitGroup
// 	for i := range numJobs {
// 		task, err := wp.Submit("job")
// 		require.NoError(t, err)
// 		jobs[i] = task.Job
//
// 		readers.Add(1)
// 		go func(j *Job) {
// 			defer readers.Done()
// 			for range 20 {
// 				_, _ = j.Snapshot()
// 			}
// 		}(jobs[i])
// 	}
//
// 	readers.Wait()
// 	wp.wg.Wait()
// }
//
// type MockSaver struct{}
//
// func (MockSaver) SaveIssue(key, title string, loe int, teamId int, started_at, finished_at int64) error {
// 	return nil
// }
// func (MockSaver) SaveIssues([]jira.Issue) (saved, updated int, err error) {
// 	return 0, 0, nil
// }
// func (MockSaver) GetTeamIdByName(string) (int, error) {
// 	return 0, nil
// }
// func (MockSaver) GetIssuesFinishedInTimeframe(teamId int, start, end int64) ([]jira.Issue, error) {
// 	return nil, nil
// }
