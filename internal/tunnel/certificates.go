package tunnel

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// CertificateResponse represents the server's response containing certificates
type CertificateResponse struct {
	CACert     string `json:"ca_cert"`
	ClientCert string `json:"client_cert"`
	ClientKey  string `json:"client_key"`
}

// FetchCertificates fetches client certificates from the server
func FetchCertificates(serverHost, token, certsDir string) error {
	// Construct API URL
	url := fmt.Sprintf("https://%s/api/v1/certificates", serverHost)

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Make request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch certificates: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned error: %s (%d)", string(body), resp.StatusCode)
	}

	// Parse response
	var certResp CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certResp); err != nil {
		return fmt.Errorf("failed to parse server response: %w", err)
	}

	// Write certificates to files
	files := map[string]string{
		"ca.crt":     certResp.CACert,
		"client.crt": certResp.ClientCert,
		"client.key": certResp.ClientKey,
	}

	for filename, content := range files {
		path := filepath.Join(certsDir, filename)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	return nil
}