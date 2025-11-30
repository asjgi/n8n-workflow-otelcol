package otel

import (
	"fmt"
	"otel-pipeline-automation/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CollectorConfig generates OpenTelemetry Collector configuration
type CollectorConfig struct {
	LokiEndpoint string
	ClusterName  string
}

// GenerateCollectorCRD creates an OpenTelemetryCollector CRD for the given request
func (c *CollectorConfig) GenerateCollectorCRD(req *models.ObservabilityRequest) (*unstructured.Unstructured, error) {
	collectorName := fmt.Sprintf("%s-otelcol", req.ServiceName)

	// Build the collector configuration
	config := map[string]interface{}{
		"receivers": map[string]interface{}{
			"otlp": map[string]interface{}{
				"protocols": map[string]interface{}{
					"grpc": map[string]interface{}{
						"endpoint": "0.0.0.0:4317",
					},
					"http": map[string]interface{}{
						"endpoint": "0.0.0.0:4318",
					},
				},
			},
		},
		"processors": map[string]interface{}{
			"batch": map[string]interface{}{},
			"resource": map[string]interface{}{
				"attributes": c.buildResourceAttributes(req),
			},
		},
		"exporters": map[string]interface{}{
			"loki": map[string]interface{}{
				"endpoint": c.LokiEndpoint,
				"labels": map[string]interface{}{
					"attributes": c.buildLokiLabels(req),
				},
			},
		},
		"service": map[string]interface{}{
			"pipelines": map[string]interface{}{
				"logs": map[string]interface{}{
					"receivers":  []string{"otlp"},
					"processors": []string{"resource", "batch"},
					"exporters":  []string{"loki"},
				},
			},
		},
	}

	// Create the OpenTelemetryCollector CRD
	collector := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "opentelemetry.io/v1beta1",
			"kind":       "OpenTelemetryCollector",
			"metadata": map[string]interface{}{
				"name":      collectorName,
				"namespace": req.Namespace,
				"labels": map[string]interface{}{
					"app.kubernetes.io/name":       collectorName,
					"app.kubernetes.io/component":  "opentelemetry-collector",
					"app.kubernetes.io/managed-by": "otel-pipeline-automation",
					"service":                      req.ServiceName,
					"team":                         req.Team,
				},
			},
			"spec": map[string]interface{}{
				"mode":   "deployment",
				"config": config,
				"replicas": 1,
				"resources": map[string]interface{}{
					"limits": map[string]interface{}{
						"memory": "512Mi",
						"cpu":    "500m",
					},
					"requests": map[string]interface{}{
						"memory": "128Mi",
						"cpu":    "100m",
					},
				},
			},
		},
	}

	collector.SetGroupVersionKind(metav1.GroupVersionKind{
		Group:   "opentelemetry.io",
		Version: "v1beta1",
		Kind:    "OpenTelemetryCollector",
	})

	return collector, nil
}

func (c *CollectorConfig) buildResourceAttributes(req *models.ObservabilityRequest) []map[string]interface{} {
	attributes := []map[string]interface{}{
		{
			"key":    "service.name",
			"value":  req.ServiceName,
			"action": "upsert",
		},
		{
			"key":    "service.namespace",
			"value":  req.Namespace,
			"action": "upsert",
		},
		{
			"key":    "team",
			"value":  req.Team,
			"action": "upsert",
		},
		{
			"key":    "cluster",
			"value":  c.ClusterName,
			"action": "upsert",
		},
	}

	// Add custom labels
	for key, value := range req.CustomLabels {
		attributes = append(attributes, map[string]interface{}{
			"key":    key,
			"value":  value,
			"action": "upsert",
		})
	}

	return attributes
}

func (c *CollectorConfig) buildLokiLabels(req *models.ObservabilityRequest) map[string]string {
	labels := map[string]string{
		"service_name": "service.name",
		"namespace":    "service.namespace",
		"team":         "team",
		"cluster":      "cluster",
		"level":        "level",
	}

	// Add custom labels
	for key := range req.CustomLabels {
		labels[key] = key
	}

	return labels
}