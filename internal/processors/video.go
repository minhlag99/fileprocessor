package processors

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// VideoProcessor processes video files
type VideoProcessor struct{}

// NewVideoProcessor creates a new video processor
func NewVideoProcessor() *VideoProcessor {
	return &VideoProcessor{}
}

// Process processes a video file
func (p *VideoProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Read the entire video into a temporary file since we can't process video streams directly
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read video: %w", err)
	}

	result := &ProcessResult{
		Metadata: make(map[string]string),
	}

	// Create a temporary file to store the video
	tempFile, err := ioutil.TempFile("", "video-*"+filepath.Ext(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err = tempFile.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write to temporary file: %w", err)
	}
	tempFile.Close() // Close to ensure data is flushed

	// Extract metadata using ffprobe if enabled and available
	if options.ExtractMetadata {
		cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", tempFile.Name())
		metadataOutput, err := cmd.Output()

		if err == nil {
			// Store the JSON output as metadata
			result.Metadata["format"] = strings.ToLower(filepath.Ext(filename))
			result.Metadata["details"] = string(metadataOutput)

			// Try to extract duration and dimensions using more direct ffprobe commands
			// Duration
			durationCmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			durationOutput, err := durationCmd.Output()
			if err == nil {
				result.Metadata["duration"] = strings.TrimSpace(string(durationOutput))
			}

			// Resolution
			widthCmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			widthOutput, err := widthCmd.Output()
			heightCmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=height", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			heightOutput, err2 := heightCmd.Output()

			if err == nil && err2 == nil {
				width := strings.TrimSpace(string(widthOutput))
				height := strings.TrimSpace(string(heightOutput))
				result.Metadata["width"] = width
				result.Metadata["height"] = height
				result.Metadata["resolution"] = width + "x" + height
			}
		}
	}

	// Generate a thumbnail preview if requested and ffmpeg is available
	if options.GeneratePreview {
		// Create a temporary file for the thumbnail
		thumbnailFile, err := ioutil.TempFile("", "thumbnail-*.jpg")
		if err != nil {
			// Continue without preview if we can't create a temp file
			fmt.Println("Warning: could not create thumbnail temp file:", err)
		} else {
			defer os.Remove(thumbnailFile.Name())
			thumbnailFile.Close()

			// Try to generate thumbnail at 5 seconds or 10% into the video
			cmd := exec.Command("ffmpeg", "-i", tempFile.Name(), "-ss", "00:00:05", "-vframes", "1", thumbnailFile.Name())
			err = cmd.Run()

			if err != nil {
				// If failed at 5 seconds, try at beginning
				cmd = exec.Command("ffmpeg", "-i", tempFile.Name(), "-vframes", "1", thumbnailFile.Name())
				err = cmd.Run()
			}

			if err == nil {
				thumbnailData, err := ioutil.ReadFile(thumbnailFile.Name())
				if err == nil {
					result.Preview = thumbnailData
				}
			}
		}
	}

	// Generate summary
	result.Summary = fmt.Sprintf("Video file: %s", filename)
	if result.Metadata["duration"] != "" {
		result.Summary += fmt.Sprintf(", Duration: %s seconds", result.Metadata["duration"])
	}
	if result.Metadata["resolution"] != "" {
		result.Summary += fmt.Sprintf(", Resolution: %s", result.Metadata["resolution"])
	}

	return result, nil
}

// CanProcess returns true if this processor can process the given content type
func (p *VideoProcessor) CanProcess(contentType, ext string) bool {
	// Check content type
	if strings.HasPrefix(contentType, "video/") {
		return true
	}

	// Check file extension
	normalizedExt := strings.ToLower(ext)
	videoExts := []string{".mp4", ".avi", ".mov", ".mkv", ".wmv", ".flv", ".webm", ".mpeg", ".mpg"}
	for _, vExt := range videoExts {
		if normalizedExt == vExt {
			return true
		}
	}

	return false
}

// init registers the processor with the registry
func init() {
	Register(NewVideoProcessor(), "video/mp4", "video/quicktime", "video/x-msvideo", "video/webm", "video/x-matroska")
}
