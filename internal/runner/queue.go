package runner

import (
	"context"
	"errors"
	"sync"
)

type Job struct {
	ID      string
	Command string
	Args    []string
	Run     func(ctx context.Context) (Result, error)
}

type Queue struct {
	limit     int
	jobs      chan Job
	results   chan Result
	wg        sync.WaitGroup
	closeOnce sync.Once
	closed    chan struct{}
}

func NewQueue(limit int) *Queue {
	if limit < 1 {
		limit = 1
	}
	return &Queue{
		limit:   limit,
		jobs:    make(chan Job),
		results: make(chan Result),
		closed:  make(chan struct{}),
	}
}

func (q *Queue) Start(ctx context.Context) {
	for i := 0; i < q.limit; i++ {
		q.wg.Add(1)
		go func() {
			defer q.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-q.jobs:
					if !ok {
						return
					}
					result, err := job.Run(ctx)
					if err != nil {
						result.Error = err.Error()
						result.Status = StatusFailed
					}
					select {
					case q.results <- result:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
}

func (q *Queue) Submit(job Job) (err error) {
	select {
	case <-q.closed:
		return errors.New("queue is stopped")
	default:
	}

	defer func() {
		if r := recover(); r != nil {
			err = errors.New("queue is stopped")
		}
	}()

	q.jobs <- job
	return err
}

func (q *Queue) Results() <-chan Result {
	return q.results
}

func (q *Queue) Stop() {
	q.closeOnce.Do(func() {
		close(q.closed)
		close(q.jobs)
		q.wg.Wait()
		close(q.results)
	})
}

// Signed-off-by: ronikoz
