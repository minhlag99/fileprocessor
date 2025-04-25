package processors

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// CSVProcessor processes CSV files
type CSVProcessor struct{}

// NewCSVProcessor creates a new CSV processor
func NewCSVProcessor() *CSVProcessor {
	return &CSVProcessor{}
}

// Process processes a CSV file
func (p *CSVProcessor) Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error) {
	// Parse CSV data
	csvReader := csv.NewReader(reader)
	
	// Read all records
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}
	
	result := &ProcessResult{
		Metadata: make(map[string]string),
		Data:     records,
	}
	
	// Generate summary
	rowCount := len(records)
	var colCount int
	if rowCount > 0 {
		colCount = len(records[0])
	}
	
	result.Summary = fmt.Sprintf("CSV file with %d rows and %d columns", rowCount, colCount)
	
	// Extract metadata
	if options.ExtractMetadata {
		result.Metadata["rows"] = fmt.Sprintf("%d", rowCount)
		result.Metadata["columns"] = fmt.Sprintf("%d", colCount)
		
		if rowCount > 0 {
			// Store header row in metadata
			result.Metadata["headers"] = strings.Join(records[0], ",")
		}
	}
	
	// Generate preview
	if options.GeneratePreview && rowCount > 0 {
		// For CSV preview, we'll just take the first few rows as a string
		maxPreviewRows := 10
		if maxPreviewRows > rowCount {
			maxPreviewRows = rowCount
		}
		
		var previewBuilder strings.Builder
		for i := 0; i < maxPreviewRows; i++ {
			previewBuilder.WriteString(strings.Join(records[i], ","))
			previewBuilder.WriteString("\n")
		}
		
		result.Preview = []byte(previewBuilder.String())
	}
	
	return result, nil
}

// CanProcess returns true if this processor can process the given content type
func (p *CSVProcessor) CanProcess(contentType, ext string) bool {
	if (contentType == "text/csv" || strings.HasSuffix(contentType, "/csv")) {
		return true
	}
	
	normalizedExt := strings.ToLower(ext)
	if normalizedExt == ".csv" {
		return true
	}
	
	return false
}

// init registers the processor with the registry
func init() {
	Register(NewCSVProcessor(), "text/csv", "application/csv")
}