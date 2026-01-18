package worker

import (
	"context"
	"sync"
	"time"

	"github.com/martinsuchenak/rackd/pkg/discovery"
	"github.com/martinsuchenak/rackd/internal/log"
	"github.com/martinsuchenak/rackd/internal/model"
)

// Compile-time interface check to ensure Scheduler implements discovery.Scheduler
var _ discovery.Scheduler = (*Scheduler)(nil)

// DiscoveryStorage interface for storage operations
type DiscoveryStorage interface {
	ListDiscoveryRules(networkID string) ([]model.DiscoveryRule, error)
	GetDiscoveryRule(id string) (*model.DiscoveryRule, error)
}

// Scheduler manages background tasks
// Implements discovery.Scheduler interface
type Scheduler struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Dependencies
	storage DiscoveryStorage
	scanner discovery.Scanner
}

// Task represents a scheduled or running task
type Task struct {
	ID          string
	Name        string
	Type        string // "recurring", "oneshot"
	Interval    time.Duration
	NextRun     time.Time
	LastRun     *time.Time
	Status      string // "pending", "running", "completed", "failed"
	Handler     TaskHandler
}

// TaskHandler is the function executed by a task
type TaskHandler func(ctx context.Context, taskID string) error

// NewScheduler creates a new scheduler
func NewScheduler(storage DiscoveryStorage, scanner discovery.Scanner) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		tasks:   make(map[string]*Task),
		running: false,
		ctx:     ctx,
		cancel:  cancel,
		storage: storage,
		scanner: scanner,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	s.running = true
	log.Info("Starting background scheduler")

	// Start scheduler goroutine
	s.wg.Add(1)
	go s.run()

	// Load and register recurring tasks from database
	s.loadRecurringTasks()
}

// Stop gracefully stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	log.Info("Stopping background scheduler")
	s.cancel()
	s.running = false
	s.wg.Wait()
}

// run is the main scheduler loop
func (s *Scheduler) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAndRunTasks()
		}
	}
}

// RegisterTask registers a new task
// Implements discovery.Scheduler interface
func (s *Scheduler) RegisterTask(task *discovery.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert discovery.Task to internal worker.Task
	interval := time.Duration(task.Interval) * time.Second
	var nextRun time.Time
	if task.NextRun > 0 {
		nextRun = time.Unix(task.NextRun, 0)
	} else {
		nextRun = time.Now().Add(interval)
	}

	var lastRun *time.Time
	if task.LastRun != nil {
		t := time.Unix(*task.LastRun, 0)
		lastRun = &t
	}

	internalTask := &Task{
		ID:       task.ID,
		Name:     task.Name,
		Type:     task.Type,
		Interval: interval,
		NextRun:  nextRun,
		LastRun:  lastRun,
		Status:   task.Status,
		Handler:  TaskHandler(task.Handler),
	}

	s.tasks[task.ID] = internalTask
	log.Info("Task registered", "task_id", task.ID, "type", task.Type, "interval", interval)
	return nil
}

// registerTaskInternal registers an internal task (for backward compatibility)
func (s *Scheduler) registerTaskInternal(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task
	log.Info("Task registered", "task_id", task.ID, "type", task.Type, "interval", task.Interval)
	return nil
}

// checkAndRunTasks checks for due tasks and executes them
func (s *Scheduler) checkAndRunTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for _, task := range s.tasks {
		if task.Status == "running" {
			continue
		}

		if now.After(task.NextRun) || now.Equal(task.NextRun) {
			s.runTask(task)
		}
	}
}

// runTask executes a task in a goroutine
func (s *Scheduler) runTask(task *Task) {
	task.Status = "running"
	now := time.Now()
	task.LastRun = &now

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		log.Info("Running task", "task_id", task.ID, "name", task.Name)

		err := task.Handler(s.ctx, task.ID)

		s.mu.Lock()
		defer s.mu.Unlock()

		if err != nil {
			task.Status = "failed"
			log.Error("Task failed", "task_id", task.ID, "error", err)
		} else {
			task.Status = "completed"
			log.Info("Task completed", "task_id", task.ID)
		}

		// Schedule next run for recurring tasks
		if task.Type == "recurring" {
			task.NextRun = time.Now().Add(task.Interval)
		}
	}()
}

// loadRecurringTasks loads discovery rules and creates tasks
func (s *Scheduler) loadRecurringTasks() {
	rules, err := s.storage.ListDiscoveryRules("")
	if err != nil {
		log.Error("Failed to load discovery rules", "error", err)
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		task := &Task{
			ID:       "discovery-" + rule.ID,
			Name:     "Discovery scan for network " + rule.NetworkID,
			Type:     "recurring",
			Interval: time.Duration(rule.ScanIntervalHours) * time.Hour,
			NextRun:  time.Now().Add(time.Duration(rule.ScanIntervalHours) * time.Hour),
			Handler:  s.createDiscoveryHandler(rule),
		}

		s.tasks[task.ID] = task
		log.Info("Loaded discovery task", "task_id", task.ID, "network_id", rule.NetworkID)
	}
}

// createDiscoveryHandler creates a handler for discovery scans
func (s *Scheduler) createDiscoveryHandler(rule model.DiscoveryRule) TaskHandler {
	return func(ctx context.Context, taskID string) error {
		return s.scanner.ScanNetwork(ctx, rule.NetworkID, &rule, nil)
	}
}
