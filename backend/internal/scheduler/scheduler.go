// Package scheduler runs recurring background jobs on their own tickers
// until the parent context is cancelled.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a named recurring unit of work.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Scheduler runs a fixed set of jobs concurrently, each on its own ticker.
type Scheduler struct {
	jobs   []Job
	logger *slog.Logger
}

func New(logger *slog.Logger, jobs ...Job) *Scheduler {
	return &Scheduler{jobs: jobs, logger: logger}
}

// Start runs every job immediately, then again on its own interval, until
// ctx is cancelled. It blocks until all job loops have exited.
func (s *Scheduler) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, job := range s.jobs {
		wg.Add(1)
		go func(job Job) {
			defer wg.Done()
			s.runLoop(ctx, job)
		}(job)
	}
	wg.Wait()
}

func (s *Scheduler) runLoop(ctx context.Context, job Job) {
	s.runOnce(ctx, job)

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx, job)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context, job Job) {
	start := time.Now()
	if err := job.Run(ctx); err != nil {
		s.logger.Error("scheduler: job failed", "job", job.Name, "error", err, "elapsed", time.Since(start))
		return
	}
	s.logger.Debug("scheduler: job finished", "job", job.Name, "elapsed", time.Since(start))
}
