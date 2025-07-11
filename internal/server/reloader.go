package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"mock-harbor/internal/config"
	"mock-harbor/internal/validation"
)

// isPortInUse checks if a port is already in use by another service
func (m *ServerManager) isPortInUse(port int, serviceName string) (string, bool) {
	for name, server := range m.serviceMap {
		if server.Port == port && name != serviceName {
			return name, true
		}
	}
	return "", false
}

// GetServerByService returns a server by its service name
func (m *ServerManager) GetServerByService(serviceName string) (*MockServer, bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	server, exists := m.serviceMap[serviceName]
	return server, exists
}

// ReloadService reloads the configuration for a specific service
func (m *ServerManager) ReloadService(serviceName, usecase string) error {
	log.Printf("Reloading configuration for service: %s, usecase: %s", serviceName, usecase)
	
	// Load service configuration
	svcCfg, err := config.LoadServiceConfig(m.ConfigRoot, serviceName)
	if err != nil {
		return fmt.Errorf("error loading service config: %w", err)
	}
	
	// Validate service configuration
	svcConfigPath := filepath.Join(m.ConfigRoot, serviceName, "config.yaml")
	validationResult := validation.ValidateServiceConfig(svcCfg, svcConfigPath)
	if !validationResult.IsValid() {
		return fmt.Errorf("service configuration validation failed: %s", validationResult.ErrorMessages())
	}
	
	// Load mock configurations
	mocks, err := config.LoadMockConfigs(m.ConfigRoot, serviceName, usecase)
	if err != nil {
		return fmt.Errorf("error loading mock configs: %w", err)
	}
	
	// Validate mock configurations
	mockConfigPath := filepath.Join(m.ConfigRoot, serviceName, "usecases", usecase, "all.json")
	validationResult = validation.ValidateMockConfigs(mocks, mockConfigPath)
	if !validationResult.IsValid() {
		return fmt.Errorf("mock configuration validation failed: %s", validationResult.ErrorMessages())
	}
	
	// Check if service exists
	existingServer, exists := m.GetServerByService(serviceName)
	
	// If server exists, stop it
	if exists {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		log.Printf("Stopping server for service %s before reloading", serviceName)
		if err := existingServer.Stop(ctx); err != nil {
			log.Printf("Warning: Error stopping server during reload: %v", err)
		}
		
		// Give the server a moment to fully stop
		time.Sleep(100 * time.Millisecond)
	}
	
	// Create new server with updated config
	mockServer := NewMockServer(serviceName, svcCfg.Port, mocks, svcCfg)
	
	// Add the server (this will replace the existing one if present)
	m.AddServer(mockServer)
	
	// Start the new server
	go func() {
		log.Printf("Starting reloaded server for %s on port %d", serviceName, svcCfg.Port)
		if err := mockServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting reloaded server %s: %v", serviceName, err)
		}
	}()
	
	log.Printf("Successfully reloaded configuration for service %s", serviceName)
	return nil
}

// ReloadGlobalConfig reloads the global configuration
func (m *ServerManager) ReloadGlobalConfig() error {
	log.Printf("Reloading global configuration...")
	
	// Load global configuration
	globalConfigPath := filepath.Join(m.ConfigRoot, "config.yaml")
	globalCfg, err := config.LoadGlobalConfig(globalConfigPath)
	if err != nil {
		return fmt.Errorf("error loading global configuration: %w", err)
	}
	
	// Validate global configuration
	validationResult := validation.ValidateGlobalConfig(globalCfg, globalConfigPath)
	if !validationResult.IsValid() {
		return fmt.Errorf("global configuration validation failed: %s", validationResult.ErrorMessages())
	}
	
	// Track current services to detect removed ones
	currentServices := make(map[string]bool)
	for _, server := range m.Servers {
		currentServices[server.ServiceName] = true
	}
	
	// Track new or updated services
	processedServices := make(map[string]bool)
	
	// Process each service in the global config
	for _, svcRef := range globalCfg.Services {
		processedServices[svcRef.Name] = true
		
		// Reload the service
		if err := m.ReloadService(svcRef.Name, svcRef.Usecase); err != nil {
			log.Printf("Error reloading service %s: %v", svcRef.Name, err)
			// Continue with other services even if this one fails
		}
	}
	
	// Stop any services that were removed from the global config
	for name := range currentServices {
		if !processedServices[name] {
			log.Printf("Service %s was removed from global config, stopping server", name)
			
			// Get the server
			server, exists := m.GetServerByService(name)
			if exists {
				// Stop the server
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := server.Stop(ctx); err != nil {
					log.Printf("Error stopping removed server %s: %v", name, err)
				}
				cancel()
				
				// Remove from manager
				m.mutex.Lock()
				delete(m.serviceMap, name)
				delete(m.portMap, server.Port)
				
				// Remove from servers slice
				for i, s := range m.Servers {
					if s.ServiceName == name {
						m.Servers = append(m.Servers[:i], m.Servers[i+1:]...)
						break
					}
				}
				m.mutex.Unlock()
			}
		}
	}
	
	log.Printf("Global configuration reloaded successfully")
	return nil
}
