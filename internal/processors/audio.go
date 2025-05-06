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

// AudioProcessor processes audio files
type AudioProcessor struct{}

// NewAudioProcessor creates a new audio processor
func NewAudioProcessor() *AudioProcessor {
	return &AudioProcessor{}
}

// Process processes an audio file
func (p *AudioProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Read the entire audio into a temporary file since we can't process audio streams directly
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	result := &ProcessResult{
		Metadata: make(map[string]string),
	}

	// Create a temporary file to store the audio
	tempFile, err := ioutil.TempFile("", "audio-*"+filepath.Ext(filename))
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

			// Try to extract duration using a more direct ffprobe command
			durationCmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			durationOutput, err := durationCmd.Output()
			if err == nil {
				result.Metadata["duration"] = strings.TrimSpace(string(durationOutput))
			}

			// Try to extract bitrate
			bitrateCmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=bit_rate", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			bitrateOutput, err := bitrateCmd.Output()
			if err == nil {
				result.Metadata["bitrate"] = strings.TrimSpace(string(bitrateOutput))
			}

			// Try to extract sample rate
			sampleRateCmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=sample_rate", "-of", "default=noprint_wrappers=1:nokey=1", tempFile.Name())
			sampleRateOutput, err := sampleRateCmd.Output()
			if err == nil {
				result.Metadata["sample_rate"] = strings.TrimSpace(string(sampleRateOutput))
			}
		}
	}

	// Generate a waveform preview if requested and ffmpeg is available
	if options.GeneratePreview {
		// Create a temporary file for the waveform image
		waveformFile, err := ioutil.TempFile("", "waveform-*.png")
		if err == nil {
			defer os.Remove(waveformFile.Name())
			waveformFile.Close()

			// Generate a waveform image using ffmpeg
			cmd := exec.Command(
				"ffmpeg", "-i", tempFile.Name(),
				"-filter_complex", "showwavespic=s=640x120:colors=#3498db",
				"-frames:v", "1",
				waveformFile.Name(),
			)
			err = cmd.Run()

			if err == nil {
				waveformData, err := ioutil.ReadFile(waveformFile.Name())
				if err == nil {
					result.Preview = waveformData
				}
			} else {
				// If waveform generation fails, store a small portion of the audio file itself
				// Up to maxPreviewSize bytes (for streaming purposes)
				if options.MaxPreviewSize > 0 && len(data) > options.MaxPreviewSize {
					result.Preview = data[:options.MaxPreviewSize]
				} else {
					result.Preview = data
				}
			}
		} else {
			// If we can't create a waveform, store a portion of the audio file itself
			if options.MaxPreviewSize > 0 && len(data) > options.MaxPreviewSize {
				result.Preview = data[:options.MaxPreviewSize]
			} else {
				result.Preview = data
			}
		}
	}

	// Generate summary
	result.Summary = fmt.Sprintf("Audio file: %s", filename)
	if result.Metadata["duration"] != "" {
		result.Summary += fmt.Sprintf(", Duration: %s seconds", result.Metadata["duration"])
	}
	if result.Metadata["bitrate"] != "" {
		bitrateKbps := fmt.Sprintf("%.1f", float64(parseIntSafe(result.Metadata["bitrate"], 0))/1000)
		result.Summary += fmt.Sprintf(", Bitrate: %s kbps", bitrateKbps)
	}
	if result.Metadata["sample_rate"] != "" {
		result.Summary += fmt.Sprintf(", Sample rate: %s Hz", result.Metadata["sample_rate"])
	}

	return result, nil
}

// Helper function to safely parse integers
func parseIntSafe(s string, defaultVal int) int {
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return defaultVal
	}
	return val
}

// CanProcess returns true if this processor can process the given content type
func (p *AudioProcessor) CanProcess(contentType, ext string) bool {
	// Check content type
	if strings.HasPrefix(contentType, "audio/") {
		return true
	}

	// Check file extension
	normalizedExt := strings.ToLower(ext)
	audioExts := []string{".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma"}
	for _, aExt := range audioExts {
		if normalizedExt == aExt {
			return true
		}
	}

	return false
}

// init registers the processor with the registry
func init() {
	Register(NewAudioProcessor(), "audio/mpeg", "audio/wav", "audio/ogg", "audio/flac", "audio/aac", "audio/mp4")
}
