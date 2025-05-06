// Package storage provides interfaces and implementations for different storage providers
package storage

import (
	"fmt"
	"log"
	"sync"
)

// Factory is responsible for creating and managing storage providers
type Factory struct {
	providers map[string]Provider
	mu        sync.RWMutex
	// Track unavailable providers
	unavailableProviders map[string]string
}

// NewStorageFactory creates a new storage factory
func NewStorageFactory() *Factory {
	return &Factory{
		providers:            make(map[string]Provider),
		unavailableProviders: make(map[string]string),
	}
}

// RegisterProvider registers a storage provider with the factory
func (f *Factory) RegisterProvider(name string, provider Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[name] = provider
}

// MarkProviderUnavailable marks a provider type as unavailable with a reason
func (f *Factory) MarkProviderUnavailable(providerType, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unavailableProviders[providerType] = reason
	log.Printf("Storage provider '%s' marked as unavailable: %s", providerType, reason)
}

// IsProviderAvailable checks if a provider type is available
func (f *Factory) IsProviderAvailable(providerType string) (bool, string) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	reason, unavailable := f.unavailableProviders[providerType]
	return !unavailable, reason
}

// CreateProvider creates a new storage provider instance based on the config
func (f *Factory) CreateProvider(providerType string, config map[string]string) (Provider, error) {
	f.mu.RLock()
	// Check if this provider has been marked as unavailable
	if reason, unavailable := f.unavailableProviders[providerType]; unavailable {
		f.mu.RUnlock()
		return nil, fmt.Errorf("%s provider is currently unavailable: %s", providerType, reason)
	}
	f.mu.RUnlock()

	var provider Provider

	switch providerType {
	case "local":
		provider = NewLocalStorage()
	case "s3", "amazon", "aws":
		provider = NewAmazonS3Storage()
	case "gcs", "google":
		provider = NewGoogleCloudStorage()
	default:
		// Check if it's a registered custom provider
		f.mu.RLock()
		p, ok := f.providers[providerType]
		f.mu.RUnlock()

		if ok {
			// Create a new instance of the same type
			provider = p
		} else {
			return nil, fmt.Errorf("unsupported storage provider type: %s", providerType)
		}
	}

	// Initialize the provider with the config
	if err := provider.Initialize(config); err != nil {
		// Mark this provider as unavailable
		f.MarkProviderUnavailable(providerType, err.Error())
		return nil, fmt.Errorf("failed to initialize %s storage provider: %w", providerType, err)
	}

	return provider, nil
}

// DefaultFactory is the default storage factory instance
var DefaultFactory = NewStorageFactory()

// CreateProvider creates a storage provider using the default factory
func CreateProvider(providerType string, config map[string]string) (Provider, error) {
	return DefaultFactory.CreateProvider(providerType, config)
}

// IsProviderAvailable checks if a provider type is available using the default factory
func IsProviderAvailable(providerType string) (bool, string) {
	return DefaultFactory.IsProviderAvailable(providerType)
}
