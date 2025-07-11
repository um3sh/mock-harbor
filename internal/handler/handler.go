package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"time"

	"mock-harbor/internal/config"
)

// MockHandler handles incoming HTTP requests and matches them to mock responses
type MockHandler struct {
	Mocks []config.MockConfig
	DelayConfig *config.DelayConfig
}

// NewMockHandler creates a new mock handler with the given mock configurations
func NewMockHandler(mocks []config.MockConfig, delayConfig *config.DelayConfig) *MockHandler {
	return &MockHandler{Mocks: mocks, DelayConfig: delayConfig}
}

// ServeHTTP implements the http.Handler interface
func (h *MockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s", r.Method, r.URL.Path)

	// Find matching mock
	mockConfig, found := h.findMatchingMock(r)
	if !found {
		log.Printf("No matching mock found for request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("No matching mock found"))
		return
	}

	// Apply configured delay if enabled
	if h.DelayConfig != nil && h.DelayConfig.Enabled {
		delay := h.calculateDelay()
		if delay > 0 {
			log.Printf("Applying delay of %d milliseconds", delay)
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	// Apply response headers
	for key, value := range mockConfig.Response.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(mockConfig.Response.StatusCode)

	// Write response body
	if mockConfig.Response.Body != nil {
		responseBody, err := json.Marshal(mockConfig.Response.Body)
		if err != nil {
			log.Printf("Error marshalling response body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(responseBody)
	}

	log.Printf("Returned mock response with status: %d", mockConfig.Response.StatusCode)
}

// findMatchingMock tries to find a mock configuration that matches the incoming request
func (h *MockHandler) findMatchingMock(r *http.Request) (config.MockConfig, bool) {
	for _, mock := range h.Mocks {
		// Match path and method
		if r.URL.Path == mock.Request.Path && r.Method == mock.Request.Method {
			// If request body is part of the matching criteria
			if mock.Request.Body != nil {
				// Read the request body
				body, err := io.ReadAll(r.Body)
				if err != nil {
					log.Printf("Error reading request body: %v", err)
					continue
				}
				// Replace the body for later use
				r.Body = io.NopCloser(bytes.NewBuffer(body))

				// Try to parse the body as JSON
				var requestBody map[string]interface{}
				if err := json.Unmarshal(body, &requestBody); err != nil {
					log.Printf("Error unmarshalling request body: %v", err)
					continue
				}

				// Check if the body matches
				if !matchesMockBody(requestBody, mock.Request.Body) {
					continue
				}
			}
			return mock, true
		}
	}
	return config.MockConfig{}, false
}

// matchesMockBody checks if the received body matches the expected body in the mock
// This implementation only checks for the specified fields in the mock
func matchesMockBody(received, expected map[string]interface{}) bool {
	for key, expectedValue := range expected {
		receivedValue, exists := received[key]
		if !exists {
			return false
		}
		
		// For nested objects, recursively check
		if expectedMap, ok := expectedValue.(map[string]interface{}); ok {
			if receivedMap, ok := receivedValue.(map[string]interface{}); ok {
				if !matchesMockBody(receivedMap, expectedMap) {
					return false
				}
				continue
			}
			return false
		}
		
		// For primitive types, do a direct comparison
		if !reflect.DeepEqual(receivedValue, expectedValue) {
			return false
		}
	}
	return true
}

// calculateDelay determines the delay duration in milliseconds based on the delay configuration
func (h *MockHandler) calculateDelay() int {
	if h.DelayConfig == nil || !h.DelayConfig.Enabled {
		return 0
	}

	// If fixed delay is specified, use that
	if h.DelayConfig.Fixed > 0 {
		return h.DelayConfig.Fixed
	}

	// If min and max are specified, use a random value in that range
	if h.DelayConfig.Min >= 0 && h.DelayConfig.Max > h.DelayConfig.Min {
		return h.DelayConfig.Min + rand.Intn(h.DelayConfig.Max-h.DelayConfig.Min+1)
	}

	return 0
}
