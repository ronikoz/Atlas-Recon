package runner

import (
	"context"
	"testing"
	"time"
)

func TestQueueLifecycle(t *testing.T) {
	q := NewQueue(2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)

	jobRan := make(chan bool)
	err := q.Submit(Job{
		ID:      "test-1",
		Command: "echo",
		Run: func(ctx context.Context) (Result, error) {
			jobRan <- true
			return Result{Status: StatusSuccess}, nil
		},
	})

	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	select {
	case <-jobRan:
		// success
	case <-time.After(1 * time.Second):
		t.Fatal("Job did not run within timeout")
	}

	q.Stop()

	// submitting to stopped queue should return an error
	err = q.Submit(Job{ID: "test-2"})
	if err == nil {
		t.Fatal("Expected error submitting to stopped queue")
	}
}
