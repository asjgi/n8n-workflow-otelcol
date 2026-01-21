package otel

import (
	"otel-pipeline-automation/pkg/models"
	"strings"
	"testing"
)

func TestGenerateServiceReceiver(t *testing.T) {
	pm := &PipelineManager{}

	tests := []struct {
		name        string
		request     *models.ObservabilityRequest
		wantContain []string
	}{
		{
			name: "basic receiver",
			request: &models.ObservabilityRequest{
				ServiceName: "my-service",
				Namespace:   "production",
			},
			wantContain: []string{
				"filelog/my-service:",
				"/var/log/pods/production_my-service_*/*/*.log",
				"start_at: end",
				"include_file_path: true",
			},
		},
		{
			name: "receiver with different namespace",
			request: &models.ObservabilityRequest{
				ServiceName: "api-server",
				Namespace:   "ns-backend",
			},
			wantContain: []string{
				"filelog/api-server:",
				"/var/log/pods/ns-backend_api-server_*/*/*.log",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pm.generateServiceReceiver(tt.request)

			for _, want := range tt.wantContain {
				if !strings.Contains(result, want) {
					t.Errorf("generateServiceReceiver() missing expected content: %s\nGot: %s", want, result)
				}
			}
		})
	}
}

func TestAddReceiverToConfig(t *testing.T) {
	pm := &PipelineManager{}

	baseConfig := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch: {}

exporters:
  loki:
    endpoint: http://loki:3100

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki]`

	receiver := `
  filelog/test-service:
    include:
      - /var/log/pods/test-ns_test-service_*/*/*.log`

	result := pm.addReceiverToConfig(baseConfig, receiver, "test-service")

	// Check if receiver was added
	if !strings.Contains(result, "filelog/test-service:") {
		t.Error("Expected receiver to be added to config")
	}

	// Check if original config is preserved
	if !strings.Contains(result, "otlp:") {
		t.Error("Original otlp receiver should be preserved")
	}

	if !strings.Contains(result, "processors:") {
		t.Error("Processors section should be preserved")
	}
}

func TestRemoveReceiverFromConfig(t *testing.T) {
	pm := &PipelineManager{}

	configWithReceiver := `receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
  filelog/test-service:
    include:
      - /var/log/pods/test-ns_test-service_*/*/*.log
    start_at: end

processors:
  batch: {}

exporters:
  loki:
    endpoint: http://loki:3100

service:
  pipelines:
    logs:
      receivers: [otlp, filelog/test-service]
      processors: [batch]
      exporters: [loki]`

	result := pm.removeReceiverFromConfig(configWithReceiver, "test-service")

	// Check if receiver was removed
	if strings.Contains(result, "filelog/test-service:") {
		t.Error("Expected filelog/test-service receiver to be removed")
	}

	// Check if original receivers are preserved
	if !strings.Contains(result, "otlp:") {
		t.Error("Original otlp receiver should be preserved")
	}

	// Check if other sections are preserved
	if !strings.Contains(result, "processors:") {
		t.Error("Processors section should be preserved")
	}

	if !strings.Contains(result, "exporters:") {
		t.Error("Exporters section should be preserved")
	}
}

func TestCollectorConfig_BuildResourceAttributes(t *testing.T) {
	config := &CollectorConfig{
		LokiEndpoint: "http://loki:3100",
		ClusterName:  "test-cluster",
	}

	req := &models.ObservabilityRequest{
		ServiceName:  "my-service",
		Namespace:    "production",
		Team:         "platform",
		CustomLabels: map[string]string{"env": "prod"},
	}

	attrs := config.buildResourceAttributes(req)

	// Check required attributes
	foundService := false
	foundNamespace := false
	foundTeam := false
	foundCluster := false
	foundCustom := false

	for _, attr := range attrs {
		key := attr["key"].(string)
		value := attr["value"].(string)

		switch key {
		case "service.name":
			foundService = true
			if value != "my-service" {
				t.Errorf("service.name = %v, want my-service", value)
			}
		case "service.namespace":
			foundNamespace = true
			if value != "production" {
				t.Errorf("service.namespace = %v, want production", value)
			}
		case "team":
			foundTeam = true
			if value != "platform" {
				t.Errorf("team = %v, want platform", value)
			}
		case "cluster":
			foundCluster = true
			if value != "test-cluster" {
				t.Errorf("cluster = %v, want test-cluster", value)
			}
		case "env":
			foundCustom = true
			if value != "prod" {
				t.Errorf("env = %v, want prod", value)
			}
		}
	}

	if !foundService {
		t.Error("Missing service.name attribute")
	}
	if !foundNamespace {
		t.Error("Missing service.namespace attribute")
	}
	if !foundTeam {
		t.Error("Missing team attribute")
	}
	if !foundCluster {
		t.Error("Missing cluster attribute")
	}
	if !foundCustom {
		t.Error("Missing custom env attribute")
	}
}

func TestCollectorConfig_BuildLokiLabels(t *testing.T) {
	config := &CollectorConfig{
		LokiEndpoint: "http://loki:3100",
		ClusterName:  "test-cluster",
	}

	req := &models.ObservabilityRequest{
		ServiceName:  "my-service",
		Namespace:    "production",
		CustomLabels: map[string]string{"env": "prod", "tier": "backend"},
	}

	labels := config.buildLokiLabels(req)

	// Check default labels
	expectedLabels := map[string]string{
		"service_name": "service.name",
		"namespace":    "service.namespace",
		"team":         "team",
		"cluster":      "cluster",
		"level":        "level",
	}

	for key, expected := range expectedLabels {
		if labels[key] != expected {
			t.Errorf("Label %s = %v, want %v", key, labels[key], expected)
		}
	}

	// Check custom labels
	if labels["env"] != "env" {
		t.Errorf("Custom label env = %v, want env", labels["env"])
	}
	if labels["tier"] != "tier" {
		t.Errorf("Custom label tier = %v, want tier", labels["tier"])
	}
}