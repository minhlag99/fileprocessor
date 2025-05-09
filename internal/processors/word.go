package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/unidoc/unioffice/document"
)

// WordProcessor processes Microsoft Word documents (.docx)
type WordProcessor struct{}

// NewWordProcessor creates a new Word document processor
func NewWordProcessor() *WordProcessor {
	return &WordProcessor{}
}

// Process processes a Word document
func (p *WordProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Read the entire document into memory
	// Note: This may not be ideal for very large documents
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read Word document: %w", err)
	}

	// Open the document
	doc, err := document.Read(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse Word document: %w", err)
	}

	result := &ProcessResult{
		Metadata: make(map[string]string),
	}

	// Extract text from the document
	var textBuilder strings.Builder
	paraCount := 0

	for _, para := range doc.Paragraphs() {
		paraCount++
		for _, run := range para.Runs() {
			textBuilder.WriteString(run.Text())
		}
		textBuilder.WriteString("\n")
	}

	extractedText := textBuilder.String()

	// Store the extracted text in the result
	result.Data = extractedText

	// Generate summary
	numChars := len(extractedText)
	numWords := len(strings.Fields(extractedText))
	result.Summary = fmt.Sprintf("Word document with %d paragraphs, %d words, and %d characters", 
		paraCount, numWords, numChars)

	// Extract metadata
	if options.ExtractMetadata {
		// Get core properties
		cp := doc.CoreProperties
		
		// Check if author/creator exists
		if creator := cp.Author(); creator != "" {
			result.Metadata["author"] = creator
		}
		
		if cp.Description() != "" {
			result.Metadata["description"] = cp.Description()
		}
		
		if cp.Title() != "" {
			result.Metadata["title"] = cp.Title()
		}
		
		// Handle date fields which are time.Time objects
		createdTime := cp.Created()
		if !createdTime.IsZero() {
			result.Metadata["created"] = createdTime.Format(time.RFC3339)
		}
		
		modifiedTime := cp.Modified()
		if !modifiedTime.IsZero() {
			result.Metadata["modified"] = modifiedTime.Format(time.RFC3339)
		}

		// Add document statistics
		result.Metadata["paragraphs"] = fmt.Sprintf("%d", paraCount)
		result.Metadata["words"] = fmt.Sprintf("%d", numWords)
		result.Metadata["characters"] = fmt.Sprintf("%d", numChars)
	}

	// Generate preview
	if options.GeneratePreview {
		maxSize := options.MaxPreviewSize
		if maxSize <= 0 {
			maxSize = 1024 // Default to 1KB
		}

		// Take the first part of the extracted text as preview
		if len(extractedText) <= maxSize {
			result.Preview = []byte(extractedText)
		} else {
			result.Preview = []byte(extractedText[:maxSize])
		}
	}

	return result, nil
}

// CanProcess returns true if this processor can process the given content type
func (p *WordProcessor) CanProcess(contentType, ext string) bool {
	// Check content type
	docxTypes := []string{
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/docx",
		"application/msword",
	}

	for _, docType := range docxTypes {
		if contentType == docType {
			return true
		}
	}

	// Check file extension
	normalizedExt := strings.ToLower(ext)
	return normalizedExt == ".docx" || normalizedExt == ".doc"
}

// init registers the processor with the registry
func init() {
	Register(
		NewWordProcessor(), 
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/docx",
		"application/msword",
	)
}

// TestWordProcessor tests the Word document processing capabilities
func TestWordProcessor(t *testing.T) {
	processor := NewWordProcessor()

	// Test with a sample Word document
	file, err := os.Open("testdata/sample.docx")
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	options := ProcessOptions{
		GeneratePreview: true,
		ExtractMetadata: true,
		MaxPreviewSize:  1024, // 1KB
	}

	result, err := processor.Process(context.Background(), file, "sample.docx", options)
	if err != nil {
		t.Fatalf("Failed to process Word document: %v", err)
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
