package processors

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
	"testing"
)

// TextProcessor processes plain text files
type TextProcessor struct{}

// NewTextProcessor creates a new text processor
func NewTextProcessor() *TextProcessor {
	return &TextProcessor{}
}

// Process processes a text file
func (p *TextProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Read the file content
	var buf bytes.Buffer
	_, err := io.Copy(&buf, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read text file: %w", err)
	}

	content := buf.String()
	
	result := &ProcessResult{
		Metadata: make(map[string]string),
		Data:     content,
	}
	
	// Count lines, words, and characters
	lineCount, wordCount, charCount := countTextStats(content)
	
	result.Summary = fmt.Sprintf("Text file with %d lines, %d words, and %d characters", lineCount, wordCount, charCount)
	
	// Extract metadata
	if options.ExtractMetadata {
		result.Metadata["lines"] = fmt.Sprintf("%d", lineCount)
		result.Metadata["words"] = fmt.Sprintf("%d", wordCount)
		result.Metadata["characters"] = fmt.Sprintf("%d", charCount)
		
		// Detect encoding (simple check for UTF-8)
		if utf8.ValidString(content) {
			result.Metadata["encoding"] = "UTF-8"
		} else {
			result.Metadata["encoding"] = "Unknown"
		}
	}
	
	// Generate preview
	if options.GeneratePreview {
		maxSize := options.MaxPreviewSize
		if maxSize <= 0 {
			maxSize = 1024 // Default to 1KB
		}
		
		// If content is smaller than max size, use it all
		if len(content) <= maxSize {
			result.Preview = []byte(content)
		} else {
			// Otherwise, take the first maxSize bytes
			result.Preview = []byte(content[:maxSize])
		}
	}
	
	return result, nil
}

// CanProcess returns true if this processor can process the given content type
func (p *TextProcessor) CanProcess(contentType, ext string) bool {
	if strings.HasPrefix(contentType, "text/") {
		return true
	}
	
	normalizedExt := strings.ToLower(ext)
	textExts := []string{".txt", ".log", ".md", ".json", ".xml", ".html", ".css", ".js"}
	for _, textExt := range textExts {
		if normalizedExt == textExt {
			return true
		}
	}
	
	return false
}

// countTextStats counts lines, words, and characters in text
func countTextStats(content string) (lines, words, chars int) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	
	// Count lines
	for scanner.Scan() {
		lines++
		lineText := scanner.Text()
		
		// Count words in this line
		wordCount := len(strings.Fields(lineText))
		words += wordCount
	}
	
	// Count characters (runes)
	chars = utf8.RuneCountInString(content)
	
	return
}

// init registers the processor with the registry
func init() {
	Register(NewTextProcessor(), "text/plain")
}

// TestTextProcessor tests the text processing capabilities
func TestTextProcessor(t *testing.T) {
	processor := NewTextProcessor()

	// Test with a sample text file
	file, err := os.Open("testdata/sample.txt")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	options := ProcessOptions{
		GeneratePreview: true,
		ExtractMetadata: true,
		MaxPreviewSize:  1024, // 1KB
	}

	result, err := processor.Process(context.Background(), file, "sample.txt", options)
	if err != nil {
		t.Fatalf("Failed to process text file: %v", err)
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
