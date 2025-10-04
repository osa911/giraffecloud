package tunnel

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandTildePath expands tilde (~) to the user's home directory
func expandTildePath(path string) string {
	// Defensive: handle empty or nil path
	if path == "" {
		return ""
	}

	if !strings.HasPrefix(path, "~") {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path // Return original path if we can't get home dir
	}

	if path == "~" {
		return homeDir
	}

	// Defensive: ensure we don't go out of bounds
	if len(path) < 2 {
		return path
	}

	return filepath.Join(homeDir, path[2:]) // Skip "~/"
}

// CertificateValidationResult holds the result of certificate validation
type CertificateValidationResult struct {
	Valid           bool
	MissingFiles    []string
	InvalidFiles    []string
	ErrorMessage    string
	SuggestedAction string
}

// ValidateCertificateFiles validates that certificate files exist and are readable
func ValidateCertificateFiles(caCertPath, clientCertPath, clientKeyPath string) *CertificateValidationResult {
	// Defensive programming: ensure we never return nil
	result := &CertificateValidationResult{
		Valid:           true,
		MissingFiles:    make([]string, 0),
		InvalidFiles:    make([]string, 0),
		SuggestedAction: "Please run 'giraffecloud login --token YOUR_TOKEN' to download certificates",
	}

	// Defensive check: handle nil result case
	if result == nil {
		return &CertificateValidationResult{
			Valid:           false,
			MissingFiles:    make([]string, 0),
			InvalidFiles:    make([]string, 0),
			ErrorMessage:    "Internal error during certificate validation",
			SuggestedAction: "Please run 'giraffecloud login --token YOUR_TOKEN' to download certificates",
		}
	}

	// Check if paths are provided
	if caCertPath == "" {
		result.Valid = false
		result.MissingFiles = append(result.MissingFiles, "CA certificate path")
	}
	if clientCertPath == "" {
		result.Valid = false
		result.MissingFiles = append(result.MissingFiles, "client certificate path")
	}
	if clientKeyPath == "" {
		result.Valid = false
		result.MissingFiles = append(result.MissingFiles, "client key path")
	}

	// If paths missing, don't check files
	if !result.Valid {
		result.ErrorMessage = fmt.Sprintf("Missing certificate configuration: %s", strings.Join(result.MissingFiles, ", "))
		return result
	}

	// Expand paths
	caCertPath = expandTildePath(caCertPath)
	clientCertPath = expandTildePath(clientCertPath)
	clientKeyPath = expandTildePath(clientKeyPath)

	// Check if files exist and are readable
	certFiles := map[string]string{
		"CA certificate":     caCertPath,
		"client certificate": clientCertPath,
		"client key":         clientKeyPath,
	}

	for name, path := range certFiles {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			result.Valid = false
			result.MissingFiles = append(result.MissingFiles, fmt.Sprintf("%s (%s)", name, path))
		} else if err != nil {
			result.Valid = false
			result.InvalidFiles = append(result.InvalidFiles, fmt.Sprintf("%s (%s): %v", name, path, err))
		}
	}

	if !result.Valid {
		var issues []string
		if len(result.MissingFiles) > 0 {
			issues = append(issues, fmt.Sprintf("Missing files: %s", strings.Join(result.MissingFiles, ", ")))
		}
		if len(result.InvalidFiles) > 0 {
			issues = append(issues, fmt.Sprintf("Invalid files: %s", strings.Join(result.InvalidFiles, ", ")))
		}
		result.ErrorMessage = strings.Join(issues, "; ")
	}

	return result
}

// CreateSecureTLSConfig creates a production-ready TLS configuration with proper certificate validation
func CreateSecureTLSConfig(caCertPath, clientCertPath, clientKeyPath string) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		},
		InsecureSkipVerify: false, // PRODUCTION: ALWAYS validate certificates
	}

	// PRODUCTION-GRADE: Validate certificates before attempting to load them
	validation := ValidateCertificateFiles(caCertPath, clientCertPath, clientKeyPath)
	if !validation.Valid {
		return nil, fmt.Errorf("CERTIFICATE VALIDATION ERROR: %s. %s", validation.ErrorMessage, validation.SuggestedAction)
	}

	// Expand tilde paths to absolute paths
	caCertPath = expandTildePath(caCertPath)
	clientCertPath = expandTildePath(clientCertPath)
	clientKeyPath = expandTildePath(clientKeyPath)

	// Load CA certificate for server validation
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("SECURITY ERROR: Failed to load CA certificate from '%s': %w\nPlease run 'giraffecloud login --token YOUR_TOKEN' to download certificates", caCertPath, err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("SECURITY ERROR: Failed to parse CA certificate from '%s'\nThe certificate file may be corrupted. Try re-running 'giraffecloud login'", caCertPath)
		}
		config.RootCAs = caCertPool
	}

	// Load client certificate for mutual TLS authentication
	if clientCertPath != "" && clientKeyPath != "" {
		clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("SECURITY ERROR: Failed to load client certificate from '%s' and '%s': %w\nPlease run 'giraffecloud login --token YOUR_TOKEN' to download certificates", clientCertPath, clientKeyPath, err)
		}
		config.Certificates = []tls.Certificate{clientCert}
	}

	return config, nil
}

// CreateSecureServerTLSConfig creates a production-ready server TLS configuration
func CreateSecureServerTLSConfig(serverCertPath, serverKeyPath, caCertPath string) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		},
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load server certificate: %w", err)
			}
			return &cert, nil
		},
	}

	// Configure client certificate validation (mutual TLS)
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		config.ClientCAs = caCertPool
		config.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return config, nil
}
