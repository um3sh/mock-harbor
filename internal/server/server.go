package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"mock-harbor/internal/config"
	"mock-harbor/internal/handler"
)

// MockServer represents a mock HTTP server for a specific service
type MockServer struct {
	ServiceName string
	Port        int
	Server      *http.Server
	Handler     *handler.MockHandler
}

// NewMockServer creates a new mock server for the given service
func NewMockServer(serviceName string, port int, mocks []config.MockConfig, serviceConfig *config.ServiceConfig) *MockServer {
	// Create delay config if service config includes it
	var delayConfig *config.DelayConfig
	if serviceConfig != nil {
		delayConfig = &serviceConfig.Delay
	}
	
	mockHandler := handler.NewMockHandler(mocks, delayConfig)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mockHandler,
	}
	
	return &MockServer{
		ServiceName: serviceName,
		Port:        port,
		Server:      server,
		Handler:     mockHandler,
	}
}

// Start begins listening for requests
func (s *MockServer) Start() error {
	log.Printf("Starting mock server for %s on port %d", s.ServiceName, s.Port)
	return s.Server.ListenAndServe()
}

// Stop gracefully shuts down the server
func (s *MockServer) Stop(ctx context.Context) error {
	log.Printf("Stopping mock server for %s", s.ServiceName)
	return s.Server.Shutdown(ctx)
}

// ServerManager manages multiple mock servers
type ServerManager struct {
	Servers     []*MockServer
	ConfigRoot  string
	serviceMap  map[string]*MockServer // Maps service names to servers
	portMap     map[int]bool           // Tracks used ports
	mutex       sync.Mutex              // Protects concurrent access during reloading
}

// NewServerManager creates a new server manager
func NewServerManager(configRoot string) *ServerManager {
	return &ServerManager{
		Servers:    make([]*MockServer, 0),
		ConfigRoot: configRoot,
		serviceMap: make(map[string]*MockServer),
		portMap:    make(map[int]bool),
	}
}

// AddServer adds a new server to the manager
func (m *ServerManager) AddServer(server *MockServer) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Check if we already have a server for this service
	if existing, exists := m.serviceMap[server.ServiceName]; exists {
		log.Printf("Replacing existing server for service %s", server.ServiceName)
		// Remove the existing server from the slice
		for i, s := range m.Servers {
			if s.ServiceName == server.ServiceName {
				m.Servers = append(m.Servers[:i], m.Servers[i+1:]...)
				break
			}
		}
		// Mark the port as free
		delete(m.portMap, existing.Port)
	}
	
	// Check if the port is already in use by a different service
	if service, inUse := m.isPortInUse(server.Port, server.ServiceName); inUse {
		log.Printf("Warning: Port %d is already in use by service %s", server.Port, service)
	}
	
	// Add the new server
	m.Servers = append(m.Servers, server)
	m.serviceMap[server.ServiceName] = server
	m.portMap[server.Port] = true
}

// StartAll starts all managed servers
func (m *ServerManager) StartAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	for _, server := range m.Servers {
		go func(s *MockServer) {
			if err := s.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("Error starting server %s: %v", s.ServiceName, err)
			}
		}(server)
	}
}

// StopAll stops all managed servers
func (m *ServerManager) StopAll() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	for _, server := range m.Servers {
		if err := server.Stop(ctx); err != nil {
			log.Printf("Error stopping server %s: %v", server.ServiceName, err)
		}
	}
}
