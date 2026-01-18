package discovery

import (
	"context"

	"github.com/martinsuchenak/rackd/internal/model"
)

// Scanner performs network discovery operations
type Scanner interface {
	// ScanNetwork scans a network based on the provided discovery rule
	// The updateFunc callback is called periodically with scan progress updates
	ScanNetwork(ctx context.Context, networkID string, rule *model.DiscoveryRule, updateFunc func(*model.DiscoveryScan)) error
}

// Scheduler manages and executes scheduled discovery tasks
type Scheduler interface {
	// Start begins the scheduler's task execution loop
	Start()

	// Stop gracefully shuts down the scheduler, waiting for all running tasks to complete
	Stop()

	// RegisterTask registers a new task with the scheduler
	RegisterTask(task *Task) error
}

// Task represents a scheduled or running discovery task
type Task struct {
	ID       string
	Name     string
	Type     string    // "recurring", "oneshot"
	Interval int       // seconds
	NextRun  int64     // unix timestamp
	LastRun  *int64    // unix timestamp
	Status   string    // "pending", "running", "completed", "failed"
	Handler  TaskHandler
}

// TaskHandler is the function executed by a task
// The context can be used for cancellation and the taskID for tracking
type TaskHandler func(ctx context.Context, taskID string) error

// ScannerFactory is a function that creates a new Scanner instance
type ScannerFactory func() (Scanner, error)

// SchedulerFactory is a function that creates a new Scheduler instance
type SchedulerFactory func(scanner Scanner) (Scheduler, error)
