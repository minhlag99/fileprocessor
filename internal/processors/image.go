package processors

import (
	"bytes"
	"context"
	"fmt"
	"image"
	// We need these imports for image.Decode to work with different formats
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// ImageProcessor processes image files
type ImageProcessor struct{}

// NewImageProcessor creates a new image processor
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{}
}

// Process processes an image file
func (p *ImageProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Read the entire image into memory
	data, err := ioutil.ReadAll(reader)
	if (err != nil) {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Decode the image
	img, format, err := image.Decode(bytes.NewReader(data))
	if (err != nil) {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	result := &ProcessResult{
		Metadata: make(map[string]string),
		Data:     img,
	}

	// Get image dimensions
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// Generate summary
	result.Summary = fmt.Sprintf("%s image, %dx%d pixels", format, width, height)

	// Extract metadata
	if options.ExtractMetadata {
		result.Metadata["width"] = fmt.Sprintf("%d", width)
		result.Metadata["height"] = fmt.Sprintf("%d", height)
		result.Metadata["format"] = format
		result.Metadata["aspectRatio"] = fmt.Sprintf("%.2f", float64(width)/float64(height))
		
		// Add file extension
		result.Metadata["extension"] = strings.ToLower(filepath.Ext(filename))
	}

	// Generate preview
	if options.GeneratePreview {
		// For preview, we'll use the original image data
		// In a real application, you might want to resize or compress it
		result.Preview = data
	}

	return result, nil
}

// CanProcess returns true if this processor can process the given content type
func (p *ImageProcessor) CanProcess(contentType, ext string) bool {
	// Check content type
	if strings.HasPrefix(contentType, "image/") {
		return true
	}

	// Check file extension
	normalizedExt := strings.ToLower(ext)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}
	for _, imgExt := range imageExts {
		if normalizedExt == imgExt {
			return true
		}
	}

	return false
}

// init registers the processor with the registry
func init() {
	Register(NewImageProcessor(), "image/jpeg", "image/png", "image/gif", "image/bmp", "image/webp")
}

// TestImageProcessor tests the image processing capabilities
func TestImageProcessor(t *testing.T) {
	processor := NewImageProcessor()

	// Test with a sample image file
	file, err := os.Open("testdata/sample.jpg")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	options := ProcessOptions{
		GeneratePreview: true,
		ExtractMetadata: true,
		MaxPreviewSize:  1024 * 10, // 10KB
	}

	result, err := processor.Process(context.Background(), file, "sample.jpg", options)
	if err != nil {
		t.Fatalf("Failed to process image file: %v", err)
	}

	// Check the result
	if result.Summary == "" {
		t.Error("Expected summary, got empty string")
	}
	if len(result.Metadata) == 0 {
		t.Error("Expected metadata, got empty map")
	}
	if len(result.Preview) == 0 {
		t.Error("Expected preview, got empty byte slice")
	}
}
