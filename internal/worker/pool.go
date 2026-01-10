package worker

import (
	"context"
	"sync"

	"github.com/martinsuchenak/rackd/internal/log"
)

// WorkerPool manages concurrent workers
type WorkerPool struct {
	maxWorkers int
	jobs       chan Job
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// Job represents a unit of work
type Job struct {
	ID      string
	Handler func(context.Context) error
	Result  chan error
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(maxWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		maxWorkers: maxWorkers,
		jobs:       make(chan Job, 100),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	for i := 0; i < p.maxWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	log.Info("Worker pool started", "workers", p.maxWorkers)
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	close(p.jobs)
	p.cancel()
	p.wg.Wait()
}

// Submit submits a job to the pool
func (p *WorkerPool) Submit(job Job) error {
	select {
	case p.jobs <- job:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// worker is the worker goroutine
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		case job, ok := <-p.jobs:
			if !ok {
				return
			}

			log.Debug("Worker executing job", "worker_id", id, "job_id", job.ID)

			err := job.Handler(p.ctx)
			if job.Result != nil {
				job.Result <- err
			}
		}
	}
}
