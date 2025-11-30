package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"otel-pipeline-automation/internal/otel"
	"otel-pipeline-automation/pkg/models"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	N8nWebhookURL     string
	PipelineManager   *otel.PipelineManager
}

// HandlePortWebhook receives webhook from Port IDP
func (h *WebhookHandler) HandlePortWebhook(c *gin.Context) {
	var portWebhook models.PortWebhook

	if err := c.ShouldBindJSON(&portWebhook); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Validate the webhook
	if portWebhook.Context.Blueprint != "observability-request" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid blueprint"})
		return
	}

	// Create n8n webhook payload
	n8nPayload := models.N8nWebhookPayload{
		RequestID: portWebhook.Context.RunID,
		Action:    portWebhook.Action.Identifier,
		Request:   portWebhook.Payload.Properties,
		Source:    "port",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Forward to n8n
	if err := h.forwardToN8n(n8nPayload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to forward to n8n",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"requestId": portWebhook.Context.RunID,
		"message":   "Observability request forwarded to automation pipeline",
	})
}

// forwardToN8n sends the payload to n8n webhook
func (h *WebhookHandler) forwardToN8n(payload models.N8nWebhookPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(h.N8nWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request to n8n: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("n8n webhook returned status code: %d", resp.StatusCode)
	}

	return nil
}

// HealthCheck endpoint for monitoring
func (h *WebhookHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"service":   "otel-pipeline-automation",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}