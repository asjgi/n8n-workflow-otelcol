package otel

import (
	"context"
	"fmt"

	"otel-pipeline-automation/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var otelCollectorGVR = schema.GroupVersionResource{
	Group:    "opentelemetry.io",
	Version:  "v1beta1",
	Resource: "opentelemetrycollectors",
}

type PipelineManager struct {
	dynamicClient      dynamic.Interface
	collectorName      string
	collectorNamespace string
}

func NewPipelineManager(dynamicClient dynamic.Interface, collectorName, collectorNamespace string) *PipelineManager {
	return &PipelineManager{
		dynamicClient:      dynamicClient,
		collectorName:      collectorName,
		collectorNamespace: collectorNamespace,
	}
}

// AddServicePipeline adds a receiver to the existing OpenTelemetryCollector CR
func (pm *PipelineManager) AddServicePipeline(ctx context.Context, req *models.ObservabilityRequest) error {
	// Get the existing OpenTelemetryCollector CR
	collector, err := pm.dynamicClient.Resource(otelCollectorGVR).Namespace(pm.collectorNamespace).Get(ctx, pm.collectorName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get OpenTelemetryCollector %s/%s: %w", pm.collectorNamespace, pm.collectorName, err)
	}

	// Get spec.config as map
	config, found, err := unstructured.NestedMap(collector.Object, "spec", "config")
	if err != nil || !found {
		return fmt.Errorf("failed to get spec.config from OpenTelemetryCollector: %w", err)
	}

	// Get or create receivers map
	receivers, found, err := unstructured.NestedMap(config, "receivers")
	if err != nil {
		return fmt.Errorf("failed to get receivers from config: %w", err)
	}
	if !found {
		receivers = make(map[string]interface{})
	}

	// Generate receiver name
	receiverName := fmt.Sprintf("filelog/%s", req.ServiceName)

	// Check if receiver already exists
	if _, exists := receivers[receiverName]; exists {
		return fmt.Errorf("receiver %s already exists", receiverName)
	}

	// Create the new receiver config
	newReceiver := pm.generateServiceReceiverMap(req)
	receivers[receiverName] = newReceiver

	// Update receivers in config
	if err := unstructured.SetNestedMap(config, receivers, "receivers"); err != nil {
		return fmt.Errorf("failed to set receivers: %w", err)
	}

	// Update pipeline to include new receiver
	if err := pm.addReceiverToPipeline(config, receiverName); err != nil {
		return fmt.Errorf("failed to add receiver to pipeline: %w", err)
	}

	// Update spec.config
	if err := unstructured.SetNestedMap(collector.Object, config, "spec", "config"); err != nil {
		return fmt.Errorf("failed to set spec.config: %w", err)
	}

	// Update the CR - OTEL Operator will automatically handle rolling update
	_, err = pm.dynamicClient.Resource(otelCollectorGVR).Namespace(pm.collectorNamespace).Update(ctx, collector, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update OpenTelemetryCollector CR: %w", err)
	}

	return nil
}

// RemoveServicePipeline removes a receiver from the OpenTelemetryCollector CR
func (pm *PipelineManager) RemoveServicePipeline(ctx context.Context, serviceName string) error {
	// Get the existing OpenTelemetryCollector CR
	collector, err := pm.dynamicClient.Resource(otelCollectorGVR).Namespace(pm.collectorNamespace).Get(ctx, pm.collectorName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get OpenTelemetryCollector: %w", err)
	}

	// Get spec.config as map
	config, found, err := unstructured.NestedMap(collector.Object, "spec", "config")
	if err != nil || !found {
		return fmt.Errorf("failed to get spec.config from OpenTelemetryCollector: %w", err)
	}

	// Get receivers map
	receivers, found, err := unstructured.NestedMap(config, "receivers")
	if err != nil || !found {
		return fmt.Errorf("failed to get receivers from config: %w", err)
	}

	// Remove the receiver
	receiverName := fmt.Sprintf("filelog/%s", serviceName)
	delete(receivers, receiverName)

	// Update receivers in config
	if err := unstructured.SetNestedMap(config, receivers, "receivers"); err != nil {
		return fmt.Errorf("failed to set receivers: %w", err)
	}

	// Remove receiver from pipeline
	if err := pm.removeReceiverFromPipeline(config, receiverName); err != nil {
		return fmt.Errorf("failed to remove receiver from pipeline: %w", err)
	}

	// Update spec.config
	if err := unstructured.SetNestedMap(collector.Object, config, "spec", "config"); err != nil {
		return fmt.Errorf("failed to set spec.config: %w", err)
	}

	// Update the CR
	_, err = pm.dynamicClient.Resource(otelCollectorGVR).Namespace(pm.collectorNamespace).Update(ctx, collector, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update OpenTelemetryCollector CR: %w", err)
	}

	return nil
}

// generateServiceReceiverMap creates filelog receiver config as a map
func (pm *PipelineManager) generateServiceReceiverMap(req *models.ObservabilityRequest) map[string]interface{} {
	return map[string]interface{}{
		"include": []interface{}{
			fmt.Sprintf("/var/log/pods/%s_%s*/%s*/*.log", req.Namespace, req.ServiceName, req.ServiceName),
		},
		"retry_on_failure": map[string]interface{}{
			"enabled":          true,
			"initial_interval": "10s",
			"max_interval":     "60s",
		},
		"include_file_name": true,
		"include_file_path": true,
		"operators": []interface{}{
			map[string]interface{}{
				"type":  "regex_parser",
				"id":    "extract_message",
				"regex": `^\d{4}-\d{2}-\d{2}T[^\s]+\s(?:stdout|stderr)\s[FPIDEW]\s(?P<message>.*)`,
			},
			map[string]interface{}{
				"type": "move",
				"if":   `attributes["message"] != nil`,
				"from": `attributes["message"]`,
				"to":   "body",
			},
			map[string]interface{}{
				"type":       "json_parser",
				"if":         `body matches "^\\\{.*\\\}$"`,
				"parse_from": "body",
				"parse_to":   `attributes["tmp"]`,
			},
			map[string]interface{}{
				"type":       "regex_parser",
				"id":         "parse_service_pod_name",
				"regex":      `/var/log/pods/(?P<namespace>[^_]+)_(?P<pod>[^_]+(?:-[^_]+)*)_[^/]+/(?P<container>[^/]+)/`,
				"parse_from": `attributes["log.file.path"]`,
			},
			map[string]interface{}{
				"type": "move",
				"if":   `attributes["namespace"] != nil`,
				"from": `attributes["namespace"]`,
				"to":   `resource["k8s.namespace.name"]`,
			},
			map[string]interface{}{
				"type": "move",
				"if":   `attributes["pod"] != nil`,
				"from": `attributes["pod"]`,
				"to":   `resource["k8s.pod.name"]`,
			},
			map[string]interface{}{
				"type": "move",
				"if":   `attributes["container"] != nil`,
				"from": `attributes["container"]`,
				"to":   `resource["k8s.container.name"]`,
			},
			map[string]interface{}{
				"type":  "add",
				"field": `resource["service.name"]`,
				"value": req.ServiceName,
			},
			map[string]interface{}{
				"type": "move",
				"if":   `attributes["tmp"] != nil and attributes["tmp"]["level"] != nil`,
				"from": "attributes.tmp.level",
				"to":   `attributes["log.severity"]`,
			},
			map[string]interface{}{
				"type": "copy",
				"if":   `attributes["log.severity"] != nil`,
				"from": `attributes["log.severity"]`,
				"to":   `attributes["level"]`,
			},
			map[string]interface{}{
				"type":  "remove",
				"if":    `attributes["tmp"] != nil`,
				"field": `attributes["tmp"]`,
			},
			map[string]interface{}{
				"type":  "remove",
				"if":    `attributes["log.file.name"] != nil`,
				"field": `attributes["log.file.name"]`,
			},
			map[string]interface{}{
				"type":  "remove",
				"if":    `attributes["log.file.path"] != nil`,
				"field": `attributes["log.file.path"]`,
			},
		},
	}
}

// addReceiverToPipeline adds a receiver to the first logs pipeline
func (pm *PipelineManager) addReceiverToPipeline(config map[string]interface{}, receiverName string) error {
	service, found, err := unstructured.NestedMap(config, "service")
	if err != nil || !found {
		return fmt.Errorf("service not found in config")
	}

	pipelines, found, err := unstructured.NestedMap(service, "pipelines")
	if err != nil || !found {
		return fmt.Errorf("pipelines not found in service")
	}

	// Find the first logs pipeline and add the receiver
	for name, pipelineVal := range pipelines {
		if name == "logs" || name == "logs/standard" || name == "logs/ntss" {
		
pipeline, ok := pipelineVal.(map[string]interface{})
			if !ok {
				continue
			}

			receivers, found, _ := unstructured.NestedStringSlice(pipeline, "receivers")
			if !found {
				// Try as interface slice
				receiversRaw, found, _ := unstructured.NestedSlice(pipeline, "receivers")
				if found {
					for _, r := range receiversRaw {
						if rs, ok := r.(string); ok {
							receivers = append(receivers, rs)
						}
					}
				}
			}

			// Add new receiver
			receivers = append(receivers, receiverName)

			// Convert back to interface slice
			receiversInterface := make([]interface{}, len(receivers))
			for i, r := range receivers {
				receiversInterface[i] = r
			}

			pipeline["receivers"] = receiversInterface
			pipelines[name] = pipeline
			break
		}
	}

	if err := unstructured.SetNestedMap(service, pipelines, "pipelines"); err != nil {
		return err
	}

	return unstructured.SetNestedMap(config, service, "service")
}

// removeReceiverFromPipeline removes a receiver from all pipelines
func (pm *PipelineManager) removeReceiverFromPipeline(config map[string]interface{}, receiverName string) error {
	service, found, err := unstructured.NestedMap(config, "service")
	if err != nil || !found {
		return fmt.Errorf("service not found in config")
	}

	pipelines, found, err := unstructured.NestedMap(service, "pipelines")
	if err != nil || !found {
		return fmt.Errorf("pipelines not found in service")
	}

	// Remove receiver from all pipelines
	for name, pipelineVal := range pipelines {
	
pipeline, ok := pipelineVal.(map[string]interface{})
		if !ok {
			continue
		}

		receiversRaw, found, _ := unstructured.NestedSlice(pipeline, "receivers")
		if !found {
			continue
		}

		// Filter out the receiver
		var newReceivers []interface{}
		for _, r := range receiversRaw {
			if rs, ok := r.(string); ok && rs != receiverName {
				newReceivers = append(newReceivers, r)
			}
		}

		pipeline["receivers"] = newReceivers
		pipelines[name] = pipeline
	}

	if err := unstructured.SetNestedMap(service, pipelines, "pipelines"); err != nil {
		return err
	}

	return unstructured.SetNestedMap(config, service, "service")
}

// GetCollectorStatus returns the status of the OpenTelemetryCollector CR
func (pm *PipelineManager) GetCollectorStatus(ctx context.Context) (map[string]interface{}, error) {
	collector, err := pm.dynamicClient.Resource(otelCollectorGVR).Namespace(pm.collectorNamespace).Get(ctx, pm.collectorName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenTelemetryCollector: %w", err)
	}

	status, _, _ := unstructured.NestedMap(collector.Object, "status")
	return status, nil
}