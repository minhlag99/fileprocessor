package processors

import "errors"

// Common errors
var (
	// ErrNoProcessor indicates that no processor is available for the given file type
	ErrNoProcessor = errors.New("no processor available for file type")

	// ErrMalformedInput indicates that the input is malformed and cannot be processed
	ErrMalformedInput = errors.New("malformed input")

	// ErrProcessingFailed indicates that processing failed for an unspecified reason
	ErrProcessingFailed = errors.New("processing failed")

	// ErrInvalidParameter indicates that an invalid parameter was passed to the processor
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrNoWorkerPool indicates that the worker pool is not initialized
	ErrNoWorkerPool = errors.New("worker pool not initialized")

	// ErrQueueFull indicates that the worker pool queue is full
	ErrQueueFull = errors.New("worker pool queue is full")

	// ErrTaskNotFound indicates that the task was not found
	ErrTaskNotFound = errors.New("task not found")

	// ErrUnsupportedFileType indicates that the file type is not supported
	ErrUnsupportedFileType = errors.New("unsupported file type")
)
