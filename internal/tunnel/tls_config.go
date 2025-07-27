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

	return filepath.Join(homeDir, path[2:]) // Skip "~/"
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

	// PRODUCTION-GRADE: Require all certificate paths for mutual TLS
	if caCertPath == "" || clientCertPath == "" || clientKeyPath == "" {
		return nil, fmt.Errorf("SECURITY ERROR: Missing certificates. Please run 'giraffecloud login --token YOUR_TOKEN' first to download certificates")
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