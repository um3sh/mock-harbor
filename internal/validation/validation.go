package validation

import (
	"fmt"
	"path/filepath"
	"strings"

	"mock-harbor/internal/config"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	File    string
	Field   string
	Message string
}

// Error returns a formatted error message
func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.File, e.Field, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Errors []ValidationError
}

// IsValid returns true if there are no validation errors
func (r ValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// ErrorMessages returns all error messages as a string
func (r ValidationResult) ErrorMessages() string {
	if r.IsValid() {
		return "Configuration valid"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation errors found:\n", len(r.Errors)))
	for i, err := range r.Errors {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// ValidateGlobalConfig validates a global configuration
func ValidateGlobalConfig(cfg *config.GlobalConfig, filePath string) ValidationResult {
	result := ValidationResult{}
	fileName := filepath.Base(filePath)

	if len(cfg.Services) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			File:    fileName,
			Field:   "services",
			Message: "no services defined, at least one service must be specified",
		})
	}

	// Validate each service reference
	serviceNames := make(map[string]bool)
	for i, service := range cfg.Services {
		fieldPrefix := fmt.Sprintf("services[%d]", i)

		// Check for empty service name
		if service.Name == "" {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   fieldPrefix + ".name",
				Message: "service name cannot be empty",
			})
		}

		// Check for empty usecase
		if service.Usecase == "" {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   fieldPrefix + ".usecase",
				Message: "usecase cannot be empty",
			})
		}

		// Check for duplicate service names
		if _, exists := serviceNames[service.Name]; exists {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   fieldPrefix + ".name",
				Message: fmt.Sprintf("duplicate service name '%s'", service.Name),
			})
		}
		serviceNames[service.Name] = true
	}

	return result
}

// ValidateServiceConfig validates a service configuration
func ValidateServiceConfig(cfg *config.ServiceConfig, filePath string) ValidationResult {
	result := ValidationResult{}
	fileName := filepath.Base(filePath)

	// Validate service name
	if cfg.Name == "" {
		result.Errors = append(result.Errors, ValidationError{
			File:    fileName,
			Field:   "name",
			Message: "service name cannot be empty",
		})
	}

	// Validate port
	if cfg.Port <= 0 {
		result.Errors = append(result.Errors, ValidationError{
			File:    fileName,
			Field:   "port",
			Message: fmt.Sprintf("invalid port number: %d, must be positive", cfg.Port),
		})
	} else if cfg.Port < 1024 || cfg.Port > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			File:    fileName,
			Field:   "port",
			Message: fmt.Sprintf("port number %d outside of recommended range (1024-65535)", cfg.Port),
		})
	}

	return result
}

// ValidateMockConfigs validates a slice of mock configurations
func ValidateMockConfigs(mocks []config.MockConfig, filePath string) ValidationResult {
	result := ValidationResult{}
	fileName := filepath.Base(filePath)

	if len(mocks) == 0 {
		result.Errors = append(result.Errors, ValidationError{
			File:    fileName,
			Field:   "",
			Message: "no mock configurations found",
		})
	}

	// Track endpoints to check for duplicates
	endpoints := make(map[string]bool)

	for i, mock := range mocks {
		mockPrefix := fmt.Sprintf("[%d]", i)

		// Validate request path
		if mock.Request.Path == "" {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   mockPrefix + ".request.path",
				Message: "path cannot be empty",
			})
		}

		// Validate request method
		if mock.Request.Method == "" {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   mockPrefix + ".request.method",
				Message: "method cannot be empty",
			})
		} else {
			validMethods := map[string]bool{
				"GET":     true,
				"POST":    true,
				"PUT":     true,
				"DELETE":  true,
				"PATCH":   true,
				"HEAD":    true,
				"OPTIONS": true,
			}
			if !validMethods[strings.ToUpper(mock.Request.Method)] {
				result.Errors = append(result.Errors, ValidationError{
					File:    fileName,
					Field:   mockPrefix + ".request.method",
					Message: fmt.Sprintf("invalid HTTP method '%s'", mock.Request.Method),
				})
			}
		}

		// Check for duplicate endpoints (same path + method)
		endpointKey := strings.ToUpper(mock.Request.Method) + ":" + mock.Request.Path
		if _, exists := endpoints[endpointKey]; exists {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   mockPrefix + ".request",
				Message: fmt.Sprintf("duplicate endpoint %s %s", mock.Request.Method, mock.Request.Path),
			})
		}
		endpoints[endpointKey] = true

		// Validate response
		if mock.Response.StatusCode < 100 || mock.Response.StatusCode > 599 {
			result.Errors = append(result.Errors, ValidationError{
				File:    fileName,
				Field:   mockPrefix + ".response.statusCode",
				Message: fmt.Sprintf("invalid HTTP status code: %d", mock.Response.StatusCode),
			})
		}
	}

	return result
}
