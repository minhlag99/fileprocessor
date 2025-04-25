// Package processors provides file processing capabilities for different file types
package processors

import (
	"context"
	"io"
	"mime"
	"path/filepath"
)

// ProcessResult contains the results of processing a file
type ProcessResult struct {
	// Summary information about the file
	Summary string 
	
	// Metadata extracted from the file
	Metadata map[string]string
	
	// Preview data that can be used to preview the file (may be nil)
	Preview []byte
	
	// Processed data (type depends on processor)
	Data interface{}
}

// ProcessOptions contains options for file processing
type ProcessOptions struct {
	// Whether to generate a preview
	GeneratePreview bool
	
	// Maximum size of preview in bytes
	MaxPreviewSize int
	
	// Whether to extract metadata
	ExtractMetadata bool
	
	// Additional processor-specific options
	Options map[string]interface{}
}

// FileProcessor is the interface that all file processors must implement
type FileProcessor interface {
	// Process processes a file and returns the result
	Process(ctx context.Context, reader io.Reader, filename string, options ProcessOptions) (*ProcessResult, error)
	
	// CanProcess returns true if this processor can process files with the given content type or extension
	CanProcess(contentType, ext string) bool
}

// GetContentTypeByExt returns the content type based on file extension
func GetContentTypeByExt(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		return "application/octet-stream"
	}
	
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	
	return contentType
}

// ProcessorRegistry maintains a registry of processors by content type
type ProcessorRegistry struct {
	processors map[string][]FileProcessor
}

// NewProcessorRegistry creates a new processor registry
func NewProcessorRegistry() *ProcessorRegistry {
	return &ProcessorRegistry{
		processors: make(map[string][]FileProcessor),
	}
}

// Register registers a processor for a specific content type
func (r *ProcessorRegistry) Register(processor FileProcessor, contentTypes ...string) {
	for _, contentType := range contentTypes {
		if _, ok := r.processors[contentType]; !ok {
			r.processors[contentType] = []FileProcessor{}
		}
		r.processors[contentType] = append(r.processors[contentType], processor)
	}
}

// GetProcessor returns a processor that can handle the given content type and extension
func (r *ProcessorRegistry) GetProcessor(contentType, ext string) FileProcessor {
	// Try direct content type match
	if processors, ok := r.processors[contentType]; ok && len(processors) > 0 {
		return processors[0]
	}
	
	// Try matching by extension
	// Convert extension to content type and try again
	extContentType := GetContentTypeByExt(ext)
	if processors, ok := r.processors[extContentType]; ok && len(processors) > 0 {
		return processors[0]
	}
	
	// Try finding a processor that can handle this type
	for _, processors := range r.processors {
		for _, processor := range processors {
			if processor.CanProcess(contentType, ext) {
				return processor
			}
		}
	}
	
	return nil
}

// DefaultRegistry is the default processor registry
var DefaultRegistry = NewProcessorRegistry()

// Register registers a processor with the default registry
func Register(processor FileProcessor, contentTypes ...string) {
	DefaultRegistry.Register(processor, contentTypes...)
}

// GetProcessor returns a processor from the default registry
func GetProcessor(contentType, ext string) FileProcessor {
	return DefaultRegistry.GetProcessor(contentType, ext)
}