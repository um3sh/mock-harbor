package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the root configuration
type GlobalConfig struct {
	Services []ServiceReference `yaml:"services"`
}

// ServiceReference points to a service to be loaded
type ServiceReference struct {
	Name    string `yaml:"name"`
	Usecase string `yaml:"usecase"`
}

// ServiceConfig represents a specific service configuration
type ServiceConfig struct {
	Port int    `yaml:"port"`
	Name string `yaml:"name"`
	Delay DelayConfig `yaml:"delay,omitempty"`
}

// DelayConfig represents configuration for simulating response latency
type DelayConfig struct {
	// Fixed delay in milliseconds for all responses
	Fixed int `yaml:"fixed,omitempty"`
	// Minimum delay in milliseconds for random delay range
	Min int `yaml:"min,omitempty"`
	// Maximum delay in milliseconds for random delay range
	Max int `yaml:"max,omitempty"`
	// Whether to enable delay for this service
	Enabled bool `yaml:"enabled,omitempty"`
}

// RequestConfig represents the request matching criteria
type RequestConfig struct {
	Path   string                 `json:"path"`
	Method string                 `json:"method"`
	Body   map[string]interface{} `json:"body,omitempty"`
}

// ResponseConfig represents the mocked response
type ResponseConfig struct {
	Body       map[string]interface{} `json:"body"`
	StatusCode int                    `json:"statusCode"`
	Headers    map[string]string      `json:"headers"`
}

// MockConfig represents a request/response pair
type MockConfig struct {
	Request  RequestConfig  `json:"request"`
	Response ResponseConfig `json:"response"`
}

// ConfigError represents an error with additional context about the configuration file
type ConfigError struct {
	FilePath string
	Message  string
	Err      error
}

// Error implements the error interface
func (e ConfigError) Error() string {
	baseMsg := fmt.Sprintf("Config error in '%s': %s", e.FilePath, e.Message)
	if e.Err != nil {
		baseMsg += ": " + e.Err.Error()
	}
	return baseMsg
}

// Unwrap returns the underlying error
func (e ConfigError) Unwrap() error {
	return e.Err
}

// LoadGlobalConfig loads and validates the global configuration file
func LoadGlobalConfig(configPath string) (*GlobalConfig, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  "global configuration file not found",
			Err:      err,
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  "error reading global config",
			Err:      err,
		}
	}

	var config GlobalConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  "error unmarshalling global config, check YAML syntax",
			Err:      err,
		}
	}

	return &config, nil
}

// LoadServiceConfig loads and validates a specific service configuration
func LoadServiceConfig(basePath, serviceName string) (*ServiceConfig, error) {
	configPath := filepath.Join(basePath, serviceName, "config.yaml")
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("service configuration for '%s' not found", serviceName),
			Err:      err,
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("error reading service config for '%s'", serviceName),
			Err:      err,
		}
	}

	var config ServiceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("error unmarshalling service config for '%s', check YAML syntax", serviceName),
			Err:      err,
		}
	}

	// Ensure config.Name matches the directory name if not explicitly set
	if config.Name == "" {
		config.Name = serviceName
	}

	return &config, nil
}

// LoadMockConfigs loads and validates mock configurations for a specific service and usecase
func LoadMockConfigs(basePath, serviceName, usecase string) ([]MockConfig, error) {
	configPath := filepath.Join(basePath, serviceName, "usecases", usecase, "all.json")
	
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("mock configurations for '%s/%s' not found", serviceName, usecase),
			Err:      err,
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("error reading mock configs for '%s/%s'", serviceName, usecase),
			Err:      err,
		}
	}

	var configs []MockConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("error unmarshalling mock configs for '%s/%s', check JSON syntax", serviceName, usecase),
			Err:      err,
		}
	}

	if len(configs) == 0 {
		return nil, &ConfigError{
			FilePath: configPath,
			Message:  fmt.Sprintf("no mock configurations found in '%s/%s'", serviceName, usecase),
		}
	}

	return configs, nil
}
