package otel

import (
	"fmt"
	"otel-pipeline-automation/pkg/models"
)

// GenerateResourceProcessor creates a resource processor for service labeling
func GenerateResourceProcessor(req *models.ObservabilityRequest) string {
	template := `
  resource/%s:
    attributes:
      - key: service.name
        value: %s
        action: upsert
      - key: service.namespace
        value: %s
        action: upsert
      - key: team
        value: %s
        action: upsert
      - key: cluster
        value: "${CLUSTER_NAME}"
        action: upsert%s`

	// Add custom labels
	customLabels := ""
	for key, value := range req.CustomLabels {
		customLabels += fmt.Sprintf(`
      - key: %s
        value: %s
        action: upsert`, key, value)
	}

	return fmt.Sprintf(template,
		req.ServiceName,
		req.ServiceName,
		req.Namespace,
		req.Team,
		customLabels)
}

// GenerateBatchProcessor creates a batch processor configuration
func GenerateBatchProcessor() string {
	return `
  batch:
    timeout: 1s
    send_batch_size: 1024
    send_batch_max_size: 2048`
}

// GenerateMemoryLimiterProcessor creates a memory limiter processor
func GenerateMemoryLimiterProcessor() string {
	return `
  memory_limiter:
    limit_mib: 512
    spike_limit_mib: 128
    check_interval: 5s`
}