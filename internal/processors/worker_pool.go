// Package processors provides file processing capabilities for different file types
package processors

import (
	"context"
	"sync"
)

// Task represents a processing task
type Task struct {
	ID         string                 // Unique identifier for the task
	ProcessorFn func() (*ProcessResult, error) // Function to execute
	Result      chan *ProcessResult    // Channel to receive the result
	Error       chan error             // Channel to receive errors
	Progress    chan int               // Channel to report progress (0-100)
	ctx         context.Context        // Context for cancellation
	cancel      context.CancelFunc     // Function to cancel the task
}

// NewTask creates a new processing task
func NewTask(id string, fn func() (*ProcessResult, error)) *Task {
	ctx, cancel := context.WithCancel(context.Background())
	return &Task{
		ID:         id,
		ProcessorFn: fn,
		Result:     make(chan *ProcessResult, 1),
		Error:      make(chan error, 1),
		Progress:   make(chan int, 10), // Buffer for progress updates
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Cancel cancels the task
func (t *Task) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}

// WorkerPool manages a pool of worker goroutines for concurrent file processing
type WorkerPool struct {
	tasks      chan *Task
	results    map[string]*Task
	numWorkers int
	wg         sync.WaitGroup
	mu         sync.RWMutex
	quit       chan struct{}
}

// NewWorkerPool creates a new worker pool with the specified number of workers
func NewWorkerPool(numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 5 // Default to 5 workers
	}

	pool := &WorkerPool{
		tasks:      make(chan *Task, 100), // Buffer for pending tasks
		results:    make(map[string]*Task),
		numWorkers: numWorkers,
		quit:       make(chan struct{}),
	}

	// Start the workers
	pool.Start()

	return pool
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	for i := 0; i < p.numWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	close(p.quit)
	p.wg.Wait()
}

// Submit submits a task to the worker pool
func (p *WorkerPool) Submit(task *Task) {
	p.mu.Lock()
	p.results[task.ID] = task
	p.mu.Unlock()

	// Send to workers
	p.tasks <- task
}

// GetTask gets a task by ID
func (p *WorkerPool) GetTask(id string) *Task {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.results[id]
}

// worker processes tasks from the queue
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case <-p.quit:
			return
		case task := <-p.tasks:
			// Execute the task
			select {
			case <-task.ctx.Done():
				// Task was cancelled
				task.Error <- context.Canceled
			default:
				// Process the task
				result, err := task.ProcessorFn()
				if err != nil {
					task.Error <- err
				} else {
					task.Result <- result
				}
			}
		}
	}
}

// DefaultPool is the default worker pool
var DefaultPool = NewWorkerPool(10)

// Submit submits a task to the default worker pool
func Submit(task *Task) {
	DefaultPool.Submit(task)
}

// GetTask gets a task from the default worker pool
func GetTask(id string) *Task {
	return DefaultPool.GetTask(id)
}