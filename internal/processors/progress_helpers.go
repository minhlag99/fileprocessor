// Package processors provides file processing capabilities for different file types
package processors

import (
	"math"
	"sync"
	"time"
)

// Cache for completed task statuses (lasts for 5 minutes)
var completedTasksCache = struct {
	sync.RWMutex
	tasks map[string]struct {
		status    map[string]interface{}
		timestamp time.Time
	}
}{tasks: make(map[string]struct {
	status    map[string]interface{}
	timestamp time.Time
})}

// StoreCompletedTaskStatus saves the final status of a completed task
func StoreCompletedTaskStatus(taskID string, status map[string]interface{}) {
	completedTasksCache.Lock()
	defer completedTasksCache.Unlock()

	// Clean up old entries first
	now := time.Now()
	for id, task := range completedTasksCache.tasks {
		if now.Sub(task.timestamp) > 5*time.Minute {
			delete(completedTasksCache.tasks, id)
		}
	}

	// Add the new task status
	completedTasksCache.tasks[taskID] = struct {
		status    map[string]interface{}
		timestamp time.Time
	}{
		status:    status,
		timestamp: now,
	}
}

// GetCompletedTaskStatus retrieves a completed task status from the cache
func GetCompletedTaskStatus(taskID string) (map[string]interface{}, bool) {
	completedTasksCache.RLock()
	defer completedTasksCache.RUnlock()

	task, exists := completedTasksCache.tasks[taskID]
	if !exists {
		return nil, false
	}

	// Check if the status is still valid
	if time.Since(task.timestamp) > 5*time.Minute {
		// Clean up expired entry
		go func() {
			completedTasksCache.Lock()
			delete(completedTasksCache.tasks, taskID)
			completedTasksCache.Unlock()
		}()
		return nil, false
	}

	return task.status, true
}

// calculateProgressFromTime estimates progress percentage based on elapsed time
func calculateProgressFromTime(elapsedSeconds float64) float64 {
	// Progress formula: fast at first, then slowing down
	// This creates a more realistic feeling progress bar

	if elapsedSeconds < 1.0 {
		// First second: 0-10%
		return elapsedSeconds * 10.0
	} else if elapsedSeconds < 5.0 {
		// 1-5 seconds: 10-50%
		return 10.0 + (elapsedSeconds-1.0)*10.0
	} else if elapsedSeconds < 15.0 {
		// 5-15 seconds: 50-80%
		return 50.0 + (elapsedSeconds-5.0)*3.0
	} else if elapsedSeconds < 30.0 {
		// 15-30 seconds: 80-90%
		return 80.0 + (elapsedSeconds-15.0)*(10.0/15.0)
	} else {
		// > 30 seconds: 90-95% (leaves room for completion event)
		return 90.0 + (5.0 * (1.0 - (30.0 / math.Max(elapsedSeconds, 30.0))))
	}
}
