package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/osa911/giraffecloud/internal/api/constants"
	"github.com/osa911/giraffecloud/internal/api/dto/common"
	"github.com/osa911/giraffecloud/internal/logging"
	"github.com/osa911/giraffecloud/internal/utils"

	"github.com/briandowns/spinner"
	"github.com/gin-gonic/gin"
)

// CertificateResponse is the response body for client certificate issuance
// (matches what the CLI expects)
type CertificateResponse struct {
	CACert     string `json:"ca_cert"`
	ClientCert string `json:"client_cert"`
	ClientKey  string `json:"client_key"`
}

// IssueClientCertificate issues a new client certificate for the authenticated user
type TunnelCertificateHandler struct{}

func NewTunnelCertificateHandler() *TunnelCertificateHandler {
	return &TunnelCertificateHandler{}
}

func (h *TunnelCertificateHandler) IssueClientCertificate(c *gin.Context) {
	logger := logging.GetGlobalLogger()
	userID := c.MustGet(constants.ContextKeyUserID).(uint32)

	// Determine certificate paths based on environment
	certDir := "/app/certs"
	env := os.Getenv("ENV")
	if env == "development" || env == "" {
		// Use local certs directory for development
		certDir = "certs"
	}

	// Load CA cert and key
	caCertPath := certDir + "/ca.crt"
	caKeyPath := certDir + "/ca.key"
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		logger.Error("Failed to read CA cert: %v", err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to read CA certificate")
		return
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		logger.Error("Failed to read CA key: %v", err)
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to read CA key")
		return
	}

	// Parse CA cert and key
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Invalid CA certificate PEM")
		return
	}
	ca, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to parse CA certificate")
		return
	}
	keyBlock, _ := pem.Decode(caKeyPEM)
	if keyBlock == nil {
		utils.HandleAPIError(c, nil, common.ErrCodeInternalServer, "Invalid CA key PEM")
		return
	}
	caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		caKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
		if err != nil {
			utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to parse CA private key")
			return
		}
	}

	// Generate client private key
	clientKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to generate client key")
		return
	}

	// Create client certificate template
	now := time.Now()
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	clientTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"GiraffeCloud"},
			CommonName:   fmt.Sprintf("giraffecloud-client-%d", userID),
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Sign client certificate
	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, ca, &clientKey.PublicKey, caKey)
	if err != nil {
		utils.HandleAPIError(c, err, common.ErrCodeInternalServer, "Failed to sign client certificate")
		return
	}

	// Encode client cert and key as PEM
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	// Respond with all certs as PEM strings
	resp := CertificateResponse{
		CACert:     string(caCertPEM),
		ClientCert: string(clientCertPEM),
		ClientKey:  string(clientKeyPEM),
	}
	c.Header("Content-Type", "application/json")
	json.NewEncoder(c.Writer).Encode(resp)
}

// FetchCertificates fetches client certificates from the API server
func FetchCertificates(apiHost string, apiPort int, token string) (*CertificateResponse, error) {
	logger := logging.GetGlobalLogger()
	logger.Info("Fetching certificates from API server: %s:%d", apiHost, apiPort)

	// Start spinner
	s := spinner.New(spinner.CharSets[14], 120*time.Millisecond)
	s.Suffix = " Downloading certificates..."
	s.Start()
	defer s.Stop()

	url := fmt.Sprintf("https://%s:%d/api/v1/tunnels/certificates", apiHost, apiPort)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch certificates (status %d): %s", resp.StatusCode, string(body))
	}

	var certResp CertificateResponse
	if err := json.NewDecoder(resp.Body).Decode(&certResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &certResp, nil
}
