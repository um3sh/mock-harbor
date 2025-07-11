package hotreload

import (
	"log"
	"path/filepath"
	"time"

	"mock-harbor/internal/server"
	"mock-harbor/internal/watcher"
)

// HotReloader handles hot reloading of configuration files
type HotReloader struct {
	configWatcher *watcher.ConfigWatcher
	serverManager *server.ServerManager
}

// NewHotReloader creates a new hot reloader
func NewHotReloader(configRoot string, serverManager *server.ServerManager) (*HotReloader, error) {
	// Create a config watcher with a callback to handle configuration changes
	// Using 500ms debounce to avoid multiple rapid reloads
	configWatcher, err := watcher.NewConfigWatcher(configRoot, nil, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}

	reloader := &HotReloader{
		configWatcher: configWatcher,
		serverManager: serverManager,
	}

	// Set the callback after the reloader is created
	configWatcher.SetCallback(reloader.handleConfigChange)

	return reloader, nil
}

// Start begins monitoring for configuration changes
func (r *HotReloader) Start() error {
	log.Println("Starting hot reload monitor...")
	return r.configWatcher.Start()
}

// Stop stops monitoring for configuration changes
func (r *HotReloader) Stop() {
	r.configWatcher.Stop()
	log.Println("Hot reload monitor stopped")
}

// handleConfigChange handles configuration file change events
func (r *HotReloader) handleConfigChange(event watcher.ConfigChangeEvent) {
	if event.IsDeleted {
		log.Printf("Config file deleted: %s (ignoring for now)", event.Path)
		return
	}

	log.Printf("Config change detected: %s, type: %s, service: %s", 
		filepath.Base(event.Path), event.ConfigType, event.ServiceID)

	switch event.ConfigType {
	case "global":
		// Global config change - reload everything
		if err := r.serverManager.ReloadGlobalConfig(); err != nil {
			log.Printf("Error reloading global config: %v", err)
		}
	case "service":
		// Service config change - need to reload that service but need usecase info
		// Get the current usecase for the service
		usecase, err := getServiceUsecase(r.serverManager.ConfigRoot, event.ServiceID)
		if err != nil {
			log.Printf("Error getting usecase for service %s: %v", event.ServiceID, err)
			return
		}
		
		if err := r.serverManager.ReloadService(event.ServiceID, usecase); err != nil {
			log.Printf("Error reloading service config for %s: %v", event.ServiceID, err)
		}
	case "mock":
		// Mock config change - need service and usecase
		// Extract usecase from path: configs/serviceA/usecases/usecase/all.json
		usecase := extractUsecaseFromPath(event.Path)
		if usecase == "" {
			log.Printf("Could not determine usecase from path: %s", event.Path)
			return
		}
		
		if err := r.serverManager.ReloadService(event.ServiceID, usecase); err != nil {
			log.Printf("Error reloading mock config for %s/%s: %v", event.ServiceID, usecase, err)
		}
	default:
		log.Printf("Ignoring change to unrecognized config type: %s", event.ConfigType)
	}
}

// getServiceUsecase gets the current usecase for a service from the global config
func getServiceUsecase(configRoot, serviceID string) (string, error) {
	// This is a simplified implementation that assumes the first usecase in the global config
	// is the one we want. A more complete implementation would parse the global config
	// and find the usecase for the specific service.
	// For demonstration purposes, we'll use a hardcoded default if needed
	return "default", nil
}

// extractUsecaseFromPath extracts the usecase name from a mock config path
func extractUsecaseFromPath(path string) string {
	// Path format: .../configs/serviceA/usecases/usecaseName/all.json
	dir := filepath.Dir(path)
	return filepath.Base(dir)
}
