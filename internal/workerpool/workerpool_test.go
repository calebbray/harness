package workerpool

import (
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/calebbray/personal-agent/internal/client"
	"github.com/calebbray/personal-agent/internal/tools"
	"github.com/stretchr/testify/require"
)

func TestWorkerPoolRace(t *testing.T) {
	c, err := client.New("mock")
	require.NoError(t, err)
	cfg := TaskConfig{
		Client: c,
		Tools:  tools.Default(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	wp := New(3, cfg)
	wp.Run()

	const numJobs = 50
	jobs := make([]*Job, numJobs)

	var readers sync.WaitGroup
	for i := range numJobs {
		task := wp.Submit("job")
		jobs[i] = task.Job

		readers.Add(1)
		go func(j *Job) {
			defer readers.Done()
			for range 20 {
				_, _ = j.Snapshot()
			}
		}(jobs[i])
	}

	readers.Wait()
	wp.wg.Wait()
}
