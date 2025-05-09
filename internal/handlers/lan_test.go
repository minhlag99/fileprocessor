package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLANDiscoveryEndpoint(t *testing.T) {
	// Initialize LAN handler
	lanHandler, err := NewLANTransferHandler()
	if err != nil {
		t.Fatalf("Failed to create LAN handler: %v", err)
	}
	defer lanHandler.Stop()

	// Create test request
	req, err := http.NewRequest("GET", "/api/lan/discover", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create response recorder
	rr := httptest.NewRecorder()

	// Perform the request
	t.Log("Performing LAN discovery request...")
	lanHandler.HandleDiscoverDevices(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Logf("Response body: %s", rr.Body.String())
		return
	}

	t.Log("Successfully tested LAN device discovery endpoint")
}
