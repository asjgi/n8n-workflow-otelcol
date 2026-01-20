package otel

import (
	"context"
	"fmt"
	"strings"
	"otel-pipeline-automation/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PipelineManager struct {
	clientset         kubernetes.Interface
	configMapName     string
	configMapNamespace string
	collectorNamespace string
}

func NewPipelineManager(clientset kubernetes.Interface, configMapName, configMapNamespace, collectorNamespace string) *PipelineManager {
	return &PipelineManager{
		clientset:         clientset,
		configMapName:     configMapName,
		configMapNamespace: configMapNamespace,
		collectorNamespace: collectorNamespace,
	}
}

// AddServicePipeline adds only receiver and connects to existing pipeline
func (pm *PipelineManager) AddServicePipeline(ctx context.Context, req *models.ObservabilityRequest) error {
	configMap, err := pm.clientset.CoreV1().ConfigMaps(pm.configMapNamespace).Get(ctx, pm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", pm.configMapNamespace, pm.configMapName, err)
	}

	currentConfig := configMap.Data["otel-collector-config.yaml"]

	// Generate only the receiver for the new service
	receiver := pm.generateServiceReceiver(req)

	// Add receiver to existing pipeline
	updatedConfig := pm.addReceiverToConfig(currentConfig, receiver, req.ServiceName)

	configMap.Data["otel-collector-config.yaml"] = updatedConfig

	_, err = pm.clientset.CoreV1().ConfigMaps(pm.configMapNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return pm.triggerDaemonSetReload(ctx)
}

// RemoveServicePipeline removes only the service receiver
func (pm *PipelineManager) RemoveServicePipeline(ctx context.Context, serviceName string) error {
	configMap, err := pm.clientset.CoreV1().ConfigMaps(pm.configMapNamespace).Get(ctx, pm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	currentConfig := configMap.Data["otel-collector-config.yaml"]
	updatedConfig := pm.removeReceiverFromConfig(currentConfig, serviceName)

	configMap.Data["otel-collector-config.yaml"] = updatedConfig

	_, err = pm.clientset.CoreV1().ConfigMaps(pm.configMapNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return pm.triggerDaemonSetReload(ctx)
}

// generateServiceReceiver creates filelog receiver for the service
func (pm *PipelineManager) generateServiceReceiver(req *models.ObservabilityRequest) string {
	template := `
  filelog/%s:
    include:
      - /var/log/pods/%s_%s_*/*/*.log
    exclude:
      - /var/log/pods/%s_%s_*/*/*previous*.log
    start_at: end
    include_file_path: true
    include_file_name: false
    operators:
      - type: router
        routes:
          - output: json_parser
            expr: 'body matches "^\\{"'
          - output: regex_parser
            expr: 'body matches "^\\d{4}-\\d{2}-\\d{2}"'
      - type: json_parser
        id: json_parser
        parse_from: body
        parse_to: attributes.parsed_fields
      - type: regex_parser
        id: regex_parser
        regex: '^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[.\d]*Z?) (?P<level>\w+) (?P<message>.*)'
        parse_from: body
        parse_to: attributes.parsed_fields
      - type: move
        from: attributes.parsed_fields.level
        to: attributes.log_level
      - type: move
        from: attributes.parsed_fields.timestamp
        to: timestamp
      - type: timestamp_parser
        parse_from: timestamp
        layout: '2006-01-02T15:04:05.000Z'
        if: 'timestamp != nil'`

	return fmt.Sprintf(template,
		req.ServiceName,                        // receiver name
		req.Namespace, req.ServiceName,         // include path
		req.Namespace, req.ServiceName)         // exclude path
}

// addReceiverToConfig adds only receiver to existing configuration
func (pm *PipelineManager) addReceiverToConfig(config, receiver, serviceName string) string {
	lines := strings.Split(config, "\n")
	var result []string

	receiversAdded := false
	pipelineUpdated := false

	for _, line := range lines {
		result = append(result, line)

		// Add to receivers section
		if strings.HasPrefix(line, "receivers:") && !receiversAdded {
			receiversAdded = true
			result = append(result, strings.Split(receiver, "\n")...)
		}

		// Update existing logs pipeline to include new receiver
		if strings.Contains(line, "logs:") && strings.Contains(line, "receivers:") && !pipelineUpdated {
			// Find the receivers line and add new receiver
			if strings.Contains(line, "[") && strings.Contains(line, "]") {
				// Replace the receivers array to include new receiver
				newReceiver := fmt.Sprintf("filelog/%s", serviceName)
				if !strings.Contains(line, newReceiver) {
					// Add new receiver to existing array
					updated := strings.Replace(line, "]", fmt.Sprintf(", %s]", newReceiver), 1)
					result[len(result)-1] = updated
					pipelineUpdated = true
				}
			}
		}
	}

	return strings.Join(result, "\n")
}

// removeReceiverFromConfig removes only the service receiver and updates pipeline
func (pm *PipelineManager) removeReceiverFromConfig(config, serviceName string) string {
	lines := strings.Split(config, "\n")
	var result []string
	skipUntilNextSection := false

	for _, line := range lines {
		// Check if this is the service receiver
		if strings.Contains(line, fmt.Sprintf("filelog/%s:", serviceName)) {
			skipUntilNextSection = true
			continue
		}

		// Check if we've reached the next section
		if skipUntilNextSection {
			if (strings.HasPrefix(line, "  ") && strings.Contains(line, ":") && !strings.HasPrefix(line, "    ")) ||
			   strings.HasPrefix(line, "processors:") ||
			   strings.HasPrefix(line, "exporters:") ||
			   strings.HasPrefix(line, "service:") {
				skipUntilNextSection = false
			} else {
				continue // Skip lines that belong to the receiver being removed
			}
		}

		// Update existing logs pipeline to remove the receiver
		if strings.Contains(line, "logs:") && strings.Contains(line, "receivers:") {
			receiverName := fmt.Sprintf("filelog/%s", serviceName)
			if strings.Contains(line, receiverName) {
				// Remove receiver from array
				updated := strings.ReplaceAll(line, fmt.Sprintf(", %s", receiverName), "")
				updated = strings.ReplaceAll(updated, fmt.Sprintf("%s, ", receiverName), "")
				updated = strings.ReplaceAll(updated, receiverName, "")
				result = append(result, updated)
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// triggerDaemonSetReload triggers a reload of the OTEL Collector DaemonSet
func (pm *PipelineManager) triggerDaemonSetReload(ctx context.Context) error {
	daemonSet, err := pm.clientset.AppsV1().DaemonSets(pm.collectorNamespace).Get(ctx, "otel-collector", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get OTEL Collector DaemonSet: %w", err)
	}

	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}

	// Update annotation to trigger pod restart and config reload
	daemonSet.Spec.Template.Annotations["config-reload-timestamp"] = metav1.Now().Format("2006-01-02T15:04:05Z07:00")

	_, err = pm.clientset.AppsV1().DaemonSets(pm.collectorNamespace).Update(ctx, daemonSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update DaemonSet for reload: %w", err)
	}

	return nil
}