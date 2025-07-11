package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigChangeEvent represents a configuration change event
type ConfigChangeEvent struct {
	Path       string
	ModTime    time.Time
	IsDeleted  bool
	ServiceID  string
	ConfigType string // "global", "service", "mock"
}

// ConfigChangeCallback is a function that is called when a configuration file changes
type ConfigChangeCallback func(event ConfigChangeEvent)

// ConfigWatcher watches configuration files for changes
type ConfigWatcher struct {
	watcher       *fsnotify.Watcher
	configRoot    string
	watchedPaths  map[string]bool
	callback      ConfigChangeCallback
	debounceDelay time.Duration
	eventMux      sync.Mutex
	recentEvents  map[string]time.Time
	done          chan struct{}
}

// SetCallback sets the callback function for configuration changes
func (cw *ConfigWatcher) SetCallback(callback ConfigChangeCallback) {
	cw.eventMux.Lock()
	defer cw.eventMux.Unlock()
	cw.callback = callback
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configRoot string, callback ConfigChangeCallback, debounceDelay time.Duration) (*ConfigWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cw := &ConfigWatcher{
		watcher:       fsWatcher,
		configRoot:    configRoot,
		watchedPaths:  make(map[string]bool),
		callback:      callback,
		debounceDelay: debounceDelay,
		recentEvents:  make(map[string]time.Time),
		done:          make(chan struct{}),
	}

	return cw, nil
}

// Start begins watching for configuration changes
func (cw *ConfigWatcher) Start() error {
	// Add the root config directory to the watcher
	if err := cw.addRecursive(cw.configRoot); err != nil {
		return err
	}

	// Start the event processing goroutine
	go cw.processEvents()

	log.Printf("Config watcher started. Monitoring directory: %s", cw.configRoot)
	return nil
}

// Stop stops the watcher
func (cw *ConfigWatcher) Stop() {
	close(cw.done)
	if cw.watcher != nil {
		cw.watcher.Close()
	}
	log.Println("Config watcher stopped")
}

// processEvents processes file system events
func (cw *ConfigWatcher) processEvents() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Skip temporary files and directories
			if strings.HasPrefix(filepath.Base(event.Name), ".") {
				continue
			}

			// Only care about write and remove operations
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}

			// Skip directories for write events
			fileInfo, err := os.Stat(event.Name)
			isDir := err == nil && fileInfo.IsDir()
			isRemoved := event.Op&fsnotify.Remove != 0

			// If a new directory is created, watch it
			if isDir && event.Op&fsnotify.Create != 0 {
				cw.addRecursive(event.Name)
				continue
			}

			// Only process if it's a YAML or JSON file
			ext := strings.ToLower(filepath.Ext(event.Name))
			if ext != ".yaml" && ext != ".yml" && ext != ".json" {
				continue
			}

			// Process the config file change
			cw.handleFileChange(event.Name, isRemoved)
		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config watcher error: %v", err)
		case <-cw.done:
			return
		}
	}
}

// handleFileChange processes a configuration file change
func (cw *ConfigWatcher) handleFileChange(path string, isRemoved bool) {
	// Get file info if it exists
	var modTime time.Time
	if !isRemoved {
		if fileInfo, err := os.Stat(path); err == nil {
			modTime = fileInfo.ModTime()
		} else {
			// File was removed between the event and our check
			isRemoved = true
		}
	}

	// Get relative path to the config root
	relPath, err := filepath.Rel(cw.configRoot, path)
	if err != nil {
		log.Printf("Error getting relative path for %s: %v", path, err)
		return
	}

	// Determine service ID and config type
	serviceID, configType := cw.classifyConfigFile(relPath)

	// Create the event
	event := ConfigChangeEvent{
		Path:       path,
		ModTime:    modTime,
		IsDeleted:  isRemoved,
		ServiceID:  serviceID,
		ConfigType: configType,
	}

	// Debounce the event
	cw.debounceEvent(event)
}

// debounceEvent debounces events for the same file
func (cw *ConfigWatcher) debounceEvent(event ConfigChangeEvent) {
	cw.eventMux.Lock()
	lastEvent, exists := cw.recentEvents[event.Path]
	now := time.Now()
	cw.recentEvents[event.Path] = now
	cw.eventMux.Unlock()

	// If the event happened too recently, schedule a check
	if exists && time.Since(lastEvent) < cw.debounceDelay {
		time.AfterFunc(cw.debounceDelay, func() {
			cw.eventMux.Lock()
			lastEventTime, stillExists := cw.recentEvents[event.Path]
			timeSinceLastEvent := time.Since(lastEventTime)
			cw.eventMux.Unlock()

			if stillExists && timeSinceLastEvent >= cw.debounceDelay {
				// No recent events for this file, process it now
				cw.callback(event)
			}
		})
	} else {
		// First event or sufficient time has passed, process it immediately
		cw.callback(event)
	}
}

// classifyConfigFile determines the service ID and config type based on the file path
func (cw *ConfigWatcher) classifyConfigFile(relPath string) (serviceID string, configType string) {
	parts := strings.Split(relPath, string(filepath.Separator))

	// Global config
	if len(parts) == 1 && (parts[0] == "config.yaml" || parts[0] == "config.yml") {
		return "", "global"
	}

	// Service config
	if len(parts) >= 2 {
		serviceID = parts[0]
		
		// Service-level config
		if len(parts) == 2 && (parts[1] == "config.yaml" || parts[1] == "config.yml") {
			return serviceID, "service"
		}
		
		// Mock configs
		if len(parts) >= 4 && parts[1] == "usecases" && strings.HasSuffix(parts[len(parts)-1], ".json") {
			return serviceID, "mock"
		}
	}

	// Unknown config type
	return serviceID, "unknown"
}

// addRecursive adds a directory and all its subdirectories to the watcher
func (cw *ConfigWatcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip hidden files and directories
		if strings.HasPrefix(filepath.Base(path), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Watch this directory
		if info.IsDir() {
			// Skip if already watched
			if _, exists := cw.watchedPaths[path]; exists {
				return nil
			}
			
			if err := cw.watcher.Add(path); err != nil {
				log.Printf("Error watching directory %s: %v", path, err)
				return nil // Continue even if there's an error
			}
			
			cw.watchedPaths[path] = true
		}
		
		return nil
	})
}
