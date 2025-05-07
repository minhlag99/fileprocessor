// Package processors provides file processing capabilities for different file types
package processors

import (
	"log"
	"sync"
	"time"
)

// Task represents a processing task
type Task struct {
	ID         string                         // Unique ID for the task
	Process    func() (*ProcessResult, error) // Function to execute
	Result     chan *ProcessResult            // Channel to receive the result
	Error      chan error                     // Channel to receive errors
	Status     string                         // Status of the task
	UpdateChan chan map[string]interface{}    // Channel for progress updates
	Timestamp  time.Time                      // When the task was created
}

// NewTask creates a new task with the given ID and process function
func NewTask(id string, process func() (*ProcessResult, error)) *Task {
	return &Task{
		ID:         id,
		Process:    process,
		Result:     make(chan *ProcessResult, 1),
		Error:      make(chan error, 1),
		Status:     "queued",
		UpdateChan: make(chan map[string]interface{}, 10),
		Timestamp:  time.Now(),
	}
}

// WorkerPool manages a pool of worker goroutines
type WorkerPool struct {
	tasks       chan *Task
	workers     int
	maxAttempts int
	wg          sync.WaitGroup
	quit        chan struct{}
	active      map[string]*Task
	mu          sync.RWMutex
}

// DefaultPool is the default worker pool used by the application
var DefaultPool *WorkerPool

// InitializeWorkerPool creates and starts the default worker pool
func InitializeWorkerPool(workers, queueSize int) {
	DefaultPool = NewWorkerPool(workers, queueSize, 3)
}

// ShutdownWorkerPool shuts down the default worker pool
func ShutdownWorkerPool() {
	if DefaultPool != nil {
		DefaultPool.Stop()
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(workers, queueSize, maxAttempts int) *WorkerPool {
	if workers <= 0 {
		workers = 1
	}
	if queueSize <= 0 {
		queueSize = 10
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	pool := &WorkerPool{
		tasks:       make(chan *Task, queueSize),
		workers:     workers,
		maxAttempts: maxAttempts,
		quit:        make(chan struct{}),
		active:      make(map[string]*Task),
	}
	pool.Start()
	return pool
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	p.wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go p.worker(i)
	}
	log.Printf("Started worker pool with %d workers", p.workers)
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	close(p.quit)
	p.wg.Wait()
	log.Println("Worker pool stopped")
}

// Submit adds a task to the pool
func (p *WorkerPool) Submit(task *Task) error {
	// Register the task
	p.mu.Lock()
	p.active[task.ID] = task
	p.mu.Unlock()

	// Submit to the queue
	select {
	case p.tasks <- task:
		return nil
	default:
		// Queue is full
		p.mu.Lock()
		delete(p.active, task.ID)
		p.mu.Unlock()
		return ErrQueueFull
	}
}

// GetTask gets a task by ID
func (p *WorkerPool) GetTask(id string) (*Task, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	task, ok := p.active[id]
	return task, ok
}

// CancelTask cancels a task by ID
func (p *WorkerPool) CancelTask(id string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.active[id]
	if ok {
		delete(p.active, id)
	}
	return ok
}

// ActiveTasks returns the number of active tasks
func (p *WorkerPool) ActiveTasks() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.active)
}

// worker processes tasks from the queue
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case task := <-p.tasks:
			log.Printf("Worker %d processing task %s", id, task.ID)

			// Process the task
			result, err := task.Process()

			// Update status
			p.mu.Lock()
			delete(p.active, task.ID)
			p.mu.Unlock()

			// Send result or error
			if err != nil {
				log.Printf("Worker %d failed task %s: %v", id, task.ID, err)
				task.Error <- err
			} else {
				log.Printf("Worker %d completed task %s", id, task.ID)
				task.Result <- result
			}
		case <-p.quit:
			log.Printf("Worker %d stopping", id)
			return
		}
	}
}

// Submit submits a task to the default worker pool
func Submit(task *Task) error {
	if DefaultPool == nil {
		return ErrNoWorkerPool
	}
	return DefaultPool.Submit(task)
}

// GetWorkerPoolStats returns the current stats of the worker pool
func GetWorkerPoolStats() (int, int) {
	if DefaultPool == nil {
		return 0, 0
	}

	activeWorkers := 0
	// Count active tasks as a proxy for active workers
	DefaultPool.mu.RLock()
	activeWorkers = len(DefaultPool.active)
	DefaultPool.mu.RUnlock()

	// Get queue size by checking channel buffer
	queueSize := len(DefaultPool.tasks)

	return activeWorkers, queueSize
}
