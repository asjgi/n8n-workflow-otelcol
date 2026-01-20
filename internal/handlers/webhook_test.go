package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestHealthCheck(t *testing.T) {
	handler := &WebhookHandler{
		N8nWebhookURL: "http://localhost:5678/webhook",
	}

	router := gin.New()
	router.GET("/health", handler.HealthCheck)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", response["status"])
	}

	if response["service"] != "otel-pipeline-automation" {
		t.Errorf("Expected service 'otel-pipeline-automation', got %v", response["service"])
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Expected timestamp field in response")
	}
}

func TestHandlePortWebhook_InvalidJSON(t *testing.T) {
	handler := &WebhookHandler{
		N8nWebhookURL: "http://localhost:5678/webhook",
	}

	router := gin.New()
	router.POST("/webhook/port", handler.HandlePortWebhook)

	req, _ := http.NewRequest("POST", "/webhook/port", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlePortWebhook_InvalidBlueprint(t *testing.T) {
	handler := &WebhookHandler{
		N8nWebhookURL: "http://localhost:5678/webhook",
	}

	router := gin.New()
	router.POST("/webhook/port", handler.HandlePortWebhook)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"entity":    "test-entity",
			"blueprint": "wrong-blueprint",
			"runId":     "run-123",
		},
		"payload": map[string]interface{}{
			"properties": map[string]interface{}{
				"service_name": "test-service",
				"namespace":    "test-ns",
			},
		},
		"action": map[string]interface{}{
			"identifier": "add-observability",
			"title":      "Add Observability",
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/webhook/port", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "Invalid blueprint" {
		t.Errorf("Expected error 'Invalid blueprint', got %v", response["error"])
	}
}

func TestHandlePortWebhook_Success(t *testing.T) {
	// Create a mock n8n server
	mockN8n := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockN8n.Close()

	handler := &WebhookHandler{
		N8nWebhookURL: mockN8n.URL,
	}

	router := gin.New()
	router.POST("/webhook/port", handler.HandlePortWebhook)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"entity":    "test-entity",
			"blueprint": "observability-request",
			"runId":     "run-123",
		},
		"payload": map[string]interface{}{
			"properties": map[string]interface{}{
				"service_name": "test-service",
				"namespace":    "test-ns",
			},
		},
		"action": map[string]interface{}{
			"identifier": "add-observability",
			"title":      "Add Observability",
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/webhook/port", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "success" {
		t.Errorf("Expected status 'success', got %v", response["status"])
	}

	if response["requestId"] != "run-123" {
		t.Errorf("Expected requestId 'run-123', got %v", response["requestId"])
	}
}

func TestHandlePortWebhook_N8nFailure(t *testing.T) {
	// Create a mock n8n server that returns error
	mockN8n := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockN8n.Close()

	handler := &WebhookHandler{
		N8nWebhookURL: mockN8n.URL,
	}

	router := gin.New()
	router.POST("/webhook/port", handler.HandlePortWebhook)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"entity":    "test-entity",
			"blueprint": "observability-request",
			"runId":     "run-123",
		},
		"payload": map[string]interface{}{
			"properties": map[string]interface{}{
				"service_name": "test-service",
				"namespace":    "test-ns",
			},
		},
		"action": map[string]interface{}{
			"identifier": "add-observability",
			"title":      "Add Observability",
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/webhook/port", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}
