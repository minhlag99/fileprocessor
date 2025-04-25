package processors

import "errors"

// Common errors
var (
	ErrNoWorkerPool = errors.New("worker pool not initialized")
	ErrQueueFull    = errors.New("task queue is full")
	ErrTaskNotFound = errors.New("task not found")
	ErrUnsupportedFileType = errors.New("unsupported file type")
)