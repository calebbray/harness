package workerpool

import "sync"

type Store struct {
	jobs map[int64]*Job
	mu   sync.RWMutex
}

func NewStore() *Store {
	return &Store{
		jobs: make(map[int64]*Job),
	}
}

func (s *Store) Add(j *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.Id] = j
}

func (s *Store) Jobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	return out
}
