package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"giraffecloud/internal/logging"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	logger *logging.Logger
	secret string
}

type GitHubWebhookPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name     string `json:"name"`
		FullName string `json:"full_name"`
	} `json:"repository"`
	Release struct {
		TagName    string `json:"tag_name"`
		Prerelease bool   `json:"prerelease"`
		Draft      bool   `json:"draft"`
	} `json:"release"`
	Action string `json:"action"`
}

func NewWebhookHandler() *WebhookHandler {
	return &WebhookHandler{
		logger: logging.GetGlobalLogger(),
		secret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
	}
}

// GitHubWebhook handles GitHub webhook events
func (h *WebhookHandler) GitHubWebhook(c *gin.Context) {
	// Verify webhook signature
	signature := c.GetHeader("X-Hub-Signature-256")
	if !h.verifySignature(c, signature) {
		h.logger.Error("Invalid webhook signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Read payload
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read webhook payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read payload"})
		return
	}

	var payload GitHubWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("Failed to parse webhook payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	h.logger.Info("Received webhook for repo: %s, ref: %s, action: %s",
		payload.Repository.FullName, payload.Ref, payload.Action)

	// Handle different webhook events
	if strings.Contains(c.GetHeader("X-GitHub-Event"), "push") {
		h.handlePushEvent(payload)
	} else if strings.Contains(c.GetHeader("X-GitHub-Event"), "release") {
		h.handleReleaseEvent(payload)
	}

	c.JSON(http.StatusOK, gin.H{"status": "received"})
}

// handlePushEvent triggers deployment for pushes to main branch
func (h *WebhookHandler) handlePushEvent(payload GitHubWebhookPayload) {
	// Only deploy on push to main branch
	if payload.Ref != "refs/heads/main" && payload.Ref != "refs/heads/master" {
		h.logger.Info("Ignoring push to branch: %s", payload.Ref)
		return
	}

	h.logger.Info("🚀 Triggering deployment for push to %s", payload.Ref)

	// Run deployment script asynchronously
	go h.runDeploymentScript()
}

// handleReleaseEvent updates client version config for releases
func (h *WebhookHandler) handleReleaseEvent(payload GitHubWebhookPayload) {
	if payload.Action != "published" || payload.Release.Draft {
		h.logger.Info("Ignoring release action: %s (draft: %v)", payload.Action, payload.Release.Draft)
		return
	}

	h.logger.Info("📦 New release published: %s (prerelease: %v)",
		payload.Release.TagName, payload.Release.Prerelease)
}

// runDeploymentScript executes the webhook deployment
func (h *WebhookHandler) runDeploymentScript() {
	h.logger.Info("🔄 Starting webhook deployment...")

	scriptPath := os.Getenv("WEBHOOK_DEPLOY_SCRIPT_PATH")
	if scriptPath == "" {
		scriptPath = "/app/scripts/webhook-deploy.sh"
	}

	// Check if script exists and is executable
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		h.logger.Error("❌ Webhook deployment script not found: %s", scriptPath)
		return
	}

	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		h.logger.Error("❌ Webhook deployment failed: %v\nOutput: %s", err, string(output))
		return
	}

	h.logger.Info("✅ Webhook deployment completed successfully")
	h.logger.Debug("Webhook deployment output: %s", string(output))
}

// verifySignature validates GitHub webhook signature
func (h *WebhookHandler) verifySignature(c *gin.Context, signature string) bool {
	if h.secret == "" {
		h.logger.Warn("No webhook secret configured, skipping signature verification")
		return true // Allow in development
	}

	if signature == "" {
		return false
	}

	// GitHub sends signature as "sha256=<hash>"
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	expectedHash := signature[7:] // Remove "sha256=" prefix

	// Read body for signature verification
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return false
	}

	// Reset body for further reading
	c.Request.Body = io.NopCloser(strings.NewReader(string(body)))

	// Calculate HMAC
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	calculatedHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedHash), []byte(calculatedHash))
}