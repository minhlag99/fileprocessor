package handlers

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"strings"
	"time"
)

// getTransferSpeed calculates the appropriate transfer speed based on device type and network conditions
func getTransferSpeed(deviceType string, networkType string) (chunkSize int, updateInterval time.Duration) {
	// Default values
	chunkSize = 32 * 1024                   // 32KB default chunk size
	updateInterval = 100 * time.Millisecond // 100ms default update interval

	// Adjust based on device type
	switch strings.ToLower(deviceType) {
	case "android":
		// Android devices need larger chunks for better throughput but more frequent updates
		chunkSize = 128 * 1024                 // 128KB for Android
		updateInterval = 50 * time.Millisecond // More frequent updates
	case "ios":
		// iOS devices can handle medium chunks
		chunkSize = 64 * 1024 // 64KB for iOS
		updateInterval = 75 * time.Millisecond
	case "desktop", "windows", "mac", "linux":
		// Desktop can handle larger chunks with less frequent updates
		chunkSize = 256 * 1024 // 256KB for desktop
		updateInterval = 200 * time.Millisecond
	}

	// Further adjust based on network type
	switch strings.ToLower(networkType) {
	case "wifi", "wifi5", "wifi6":
		// WiFi can handle larger chunks
		chunkSize = int(float64(chunkSize) * 1.5)
	case "ethernet", "lan":
		// Wired connections can handle even larger chunks
		chunkSize = int(float64(chunkSize) * 2.0)
	case "4g", "lte":
		// Mobile networks need smaller chunks for reliability
		chunkSize = int(float64(chunkSize) * 0.8)
	case "3g", "slow":
		// Slow connections need much smaller chunks
		chunkSize = int(float64(chunkSize) * 0.5)
		updateInterval = 300 * time.Millisecond // Less frequent updates to reduce overhead
	}

	// Ensure reasonable bounds
	chunkSize = int(math.Max(8*1024, math.Min(float64(chunkSize), 1024*1024))) // Between 8KB and 1MB

	// Log the optimized settings
	log.Printf("Optimized transfer settings for %s on %s network: chunk size=%d bytes, update interval=%v",
		deviceType, networkType, chunkSize, updateInterval)

	return chunkSize, updateInterval
}

// For legacy compatibility
func getTransferSpeedLegacy(chunkSize int64, isAndroid bool) int64 {
	// Calculate simulated bytes per second
	if isAndroid {
		// Maximum speed for Android (around 50-100 MB/s depending on device)
		return chunkSize * 20
	}
	return chunkSize * 10
}

// detectDeviceType attempts to determine the device type from the user agent string
func detectDeviceType(userAgent string) string {
	userAgent = strings.ToLower(userAgent)

	if strings.Contains(userAgent, "android") {
		return "android"
	} else if strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad") || strings.Contains(userAgent, "ipod") {
		return "ios"
	} else if strings.Contains(userAgent, "windows") {
		return "windows"
	} else if strings.Contains(userAgent, "macintosh") || strings.Contains(userAgent, "mac os") {
		return "mac"
	} else if strings.Contains(userAgent, "linux") {
		return "linux"
	}

	// Default to the current system's OS
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "mac"
	case "linux":
		return "linux"
	default:
		return "desktop"
	}
}

// estimateRemainingTime calculates an estimated completion time based on current progress and speed
func estimateRemainingTime(totalBytes int64, processedBytes int64, bytesPerSecond float64) time.Duration {
	if bytesPerSecond <= 0 || processedBytes >= totalBytes {
		return 0
	}

	remainingBytes := totalBytes - processedBytes
	secondsRemaining := float64(remainingBytes) / bytesPerSecond

	return time.Duration(secondsRemaining * float64(time.Second))
}

// formatTransferSpeed formats a transfer speed in bytes per second to a human-readable string
func formatTransferSpeed(bytesPerSecond float64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	if bytesPerSecond < KB {
		return fmt.Sprintf("%.1f B/s", bytesPerSecond)
	} else if bytesPerSecond < MB {
		return fmt.Sprintf("%.1f KB/s", bytesPerSecond/KB)
	} else if bytesPerSecond < GB {
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	} else {
		return fmt.Sprintf("%.2f GB/s", bytesPerSecond/GB)
	}
}

// GetOptimizedBufferSize returns an optimized buffer size for the specific device and network
func GetOptimizedBufferSize(deviceType string, networkType string) int {
	chunkSize, _ := getTransferSpeed(deviceType, networkType)
	return chunkSize
}

// retryWithBackoff runs a function with exponential backoff retries
func retryWithBackoff(maxAttempts int, operation func() error) error {
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = operation()
		if err == nil {
			return nil
		}

		// Calculate backoff delay: 2^attempt * 100ms with some jitter
		backoffMs := math.Pow(2, float64(attempt)) * 100
		jitterMs := float64(50) * (1 + math.Sin(float64(time.Now().UnixNano()))) // Add some jitter (0-100ms)
		delayMs := backoffMs + jitterMs

		// Cap at 5 seconds
		if delayMs > 5000 {
			delayMs = 5000
		}

		log.Printf("Operation failed (attempt %d/%d): %v. Retrying in %.0fms",
			attempt+1, maxAttempts, err, delayMs)
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxAttempts, err)
}
