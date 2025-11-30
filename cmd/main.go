package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otel-pipeline-automation/internal/handlers"
	"otel-pipeline-automation/internal/k8s"
	"otel-pipeline-automation/pkg/models"

	"github.com/gin-gonic/gin"
)

func main() {
	// Configuration from environment variables
	port := getEnv("PORT", "8080")
	n8nWebhookURL := getEnv("N8N_WEBHOOK_URL", "http://localhost:5678/webhook/otel-automation")
	kubeconfigPath := getEnv("KUBECONFIG", "")
	lokiEndpoint := getEnv("LOKI_ENDPOINT", "http://loki:3100/loki/api/v1/push")
	clusterName := getEnv("CLUSTER_NAME", "default")

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient(kubeconfigPath, lokiEndpoint, clusterName)
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Initialize Pipeline manager for existing OTEL Collector DaemonSet
	configMapName := getEnv("OTEL_CONFIG_MAP_NAME", "otel-collector-config")
	configMapNamespace := getEnv("OTEL_CONFIG_MAP_NAMESPACE", "observability")
	collectorNamespace := getEnv("OTEL_COLLECTOR_NAMESPACE", "observability")

	pipelineManager := otel.NewPipelineManager(k8sClient.GetClientset(), configMapName, configMapNamespace, collectorNamespace)

	// Initialize handlers
	webhookHandler := &handlers.WebhookHandler{
		N8nWebhookURL:   n8nWebhookURL,
		PipelineManager: pipelineManager,
	}

	// Setup Gin router
	r := gin.Default()

	// Add middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Routes
	v1 := r.Group("/api/v1")
	{
		v1.POST("/webhook/port", webhookHandler.HandlePortWebhook)
		v1.GET("/health", webhookHandler.HealthCheck)
		v1.POST("/otel/pipeline/add", handleAddPipeline(pipelineManager))
		v1.DELETE("/otel/pipeline/:service", handleRemovePipeline(pipelineManager))
		v1.GET("/otel/status/:service/:namespace", handleOtelStatus(k8sClient))
	}

	// Setup HTTP server
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

func handleAddPipeline(pipelineManager *otel.PipelineManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.ObservabilityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload"})
			return
		}

		ctx := context.Background()
		if err := pipelineManager.AddServicePipeline(ctx, &req); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to add service pipeline",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":    "success",
			"message":   "Service receiver added to OTEL Collector pipeline",
			"service":   req.ServiceName,
			"namespace": req.Namespace,
			"receiver":  fmt.Sprintf("filelog/%s", req.ServiceName),
			"log_path":  fmt.Sprintf("/var/log/pods/%s_%s_*/", req.Namespace, req.ServiceName),
		})
	}
}

func handleOtelStatus(k8sClient *k8s.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName := c.Param("service")
		namespace := c.Param("namespace")

		ctx := context.Background()
		status, err := k8sClient.GetDaemonSetStatus(ctx, "otel-collector", namespace)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "OTEL Collector DaemonSet not found",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":     "success",
			"service":    serviceName,
			"daemonset":  status,
			"message":    "OTEL Collector DaemonSet status retrieved",
		})
	}
}

func handleRemovePipeline(pipelineManager *otel.PipelineManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		serviceName := c.Param("service")

		ctx := context.Background()
		if err := pipelineManager.RemoveServicePipeline(ctx, serviceName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to remove service pipeline",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Service receiver removed from OTEL Collector pipeline",
			"service": serviceName,
		})
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}