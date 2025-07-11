package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"mock-harbor/internal/config"
	"mock-harbor/internal/hotreload"
	"mock-harbor/internal/server"
	"mock-harbor/internal/validation"
)

// printBanner prints the application banner
func printBanner() {
	banner := `
  __  __            _       _   _            _                
 |  \/  | ___   ___| | __  | | | | __ _ _ __| |__   ___  _ __ 
 | |\/| |/ _ \ / __| |/ /  | |_| |/ _\ | '__| '_ \ / _ \| '__|
 | |  | | (_) | (__|   <   |  _  | (_| | |  | |_) | (_) | |   
 |_|  |_|\___/ \___|_|\_\  |_| |_|\__,_|_|  |_.__/ \___/|_|   

HTTP Mock Server - v1.0.0
`
	fmt.Print(banner)
}

// validateConfigDir checks if the config directory exists and has the expected structure
func validateConfigDir(configDir string) error {
	// Check if directory exists
	info, err := os.Stat(configDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("configuration directory '%s' does not exist", configDir)
	}
	if err != nil {
		return fmt.Errorf("error accessing configuration directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("'%s' is not a directory", configDir)
	}
	
	// Check for global config file
	globalConfigPath := filepath.Join(configDir, "config.yaml")
	if _, err := os.Stat(globalConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("global config file '%s' not found", globalConfigPath)
	}
	
	return nil
}

func main() {
	// Print banner
	printBanner()
	
	// Parse command line flags
	configDir := flag.String("config-dir", "configs", "Directory containing configuration files")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	disableHotReload := flag.Bool("no-hot-reload", false, "Disable hot reloading of configuration files")
	flag.Parse()

	// Resolve absolute path to config directory
	absConfigDir, err := filepath.Abs(*configDir)
	if err != nil {
		log.Fatalf("Error resolving config directory path: %v", err)
	}
	
	// Validate config directory structure
	if err := validateConfigDir(absConfigDir); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}
	
	log.Printf("Using configuration directory: %s", absConfigDir)

	// Load global configuration
	globalConfigPath := filepath.Join(absConfigDir, "config.yaml")
	log.Printf("Loading global configuration from %s", globalConfigPath)

	globalCfg, err := config.LoadGlobalConfig(globalConfigPath)
	if err != nil {
		log.Fatalf("Error loading global configuration: %v", err)
	}

	// Validate global configuration
	validationResult := validation.ValidateGlobalConfig(globalCfg, globalConfigPath)
	if !validationResult.IsValid() {
		log.Printf("Configuration validation errors found:")
		for _, err := range validationResult.Errors {
			log.Printf("  - %s", err.Error())
		}
		log.Fatalf("Please fix the configuration errors and try again.")
	}

	// Create server manager with config root
	manager := server.NewServerManager(absConfigDir)

	// Process each service
	for _, svcRef := range globalCfg.Services {
		log.Printf("Processing service: %s with usecase: %s", svcRef.Name, svcRef.Usecase)

		// Load service configuration
		svcCfg, err := config.LoadServiceConfig(absConfigDir, svcRef.Name)
		if err != nil {
			log.Printf("Error loading service config for %s: %v", svcRef.Name, err)
			continue
		}
		
		// Validate service configuration
		svcConfigPath := filepath.Join(absConfigDir, svcRef.Name, "config.yaml")
		validationResult := validation.ValidateServiceConfig(svcCfg, svcConfigPath)
		if !validationResult.IsValid() {
			log.Printf("Service '%s' configuration validation errors:", svcRef.Name)
			for _, err := range validationResult.Errors {
				log.Printf("  - %s", err.Error())
			}
			log.Printf("Skipping service '%s' due to configuration errors.", svcRef.Name)
			continue
		}

		// Load mock configurations
		mocks, err := config.LoadMockConfigs(absConfigDir, svcRef.Name, svcRef.Usecase)
		if err != nil {
			log.Printf("Error loading mock configs for %s/%s: %v", svcRef.Name, svcRef.Usecase, err)
			continue
		}
		
		// Validate mock configurations
		mockConfigPath := filepath.Join(absConfigDir, svcRef.Name, "usecases", svcRef.Usecase, "all.json")
		validationResult = validation.ValidateMockConfigs(mocks, mockConfigPath)
		if !validationResult.IsValid() {
			log.Printf("Mock configurations for '%s/%s' validation errors:", svcRef.Name, svcRef.Usecase)
			for _, err := range validationResult.Errors {
				log.Printf("  - %s", err.Error())
			}
			log.Printf("Skipping service '%s' due to mock configuration errors.", svcRef.Name)
			continue
		}

		// Create and add server
		mockServer := server.NewMockServer(svcRef.Name, svcCfg.Port, mocks, svcCfg)
		manager.AddServer(mockServer)
	}

	// Check if we have any servers to start
	if len(manager.Servers) == 0 {
		log.Fatalf("No valid mock servers configured. Please check your configuration.")
	}
	
	// Print server information
	log.Printf("Starting %d mock servers:", len(manager.Servers))
	for _, srv := range manager.Servers {
		log.Printf("  - %s on port %d", srv.ServiceName, srv.Port)
	}

	// Start all servers
	manager.StartAll()
	log.Println("All mock servers started successfully")
	
	// Set up hot reloading if enabled
	var reloader *hotreload.HotReloader
	if !*disableHotReload {
		log.Println("Initializing hot reload monitor for configuration files...")
		reloader, err = hotreload.NewHotReloader(absConfigDir, manager)
		if err != nil {
			log.Printf("Warning: Could not initialize hot reloading: %v", err)
		} else {
			if err := reloader.Start(); err != nil {
				log.Printf("Warning: Could not start hot reloading: %v", err)
			} else {
				log.Println("Hot reload monitor started successfully - changes to config files will be applied automatically")
			}
		}
	}
	
	if *verbose {
		log.Println("Server is running in verbose mode. All requests will be logged.")
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Stop hot reloader if active
	if reloader != nil {
		log.Println("Stopping hot reload monitor...")
		reloader.Stop()
	}

	// Stop all servers gracefully
	log.Println("Shutting down all mock servers...")
	manager.StopAll()
	log.Println("All servers stopped. Goodbye!")
}
