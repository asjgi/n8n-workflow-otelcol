package otel

import (
	"context"
	"fmt"
	"otel-pipeline-automation/pkg/models"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DaemonSetManager struct {
	clientset         kubernetes.Interface
	configMapName     string
	configMapNamespace string
	collectorNamespace string
}

func NewDaemonSetManager(clientset kubernetes.Interface, configMapName, configMapNamespace, collectorNamespace string) *DaemonSetManager {
	return &DaemonSetManager{
		clientset:         clientset,
		configMapName:     configMapName,
		configMapNamespace: configMapNamespace,
		collectorNamespace: collectorNamespace,
	}
}

// AddServicePipeline adds a new service pipeline to the existing DaemonSet configuration
func (dm *DaemonSetManager) AddServicePipeline(ctx context.Context, req *models.ObservabilityRequest) error {
	// Get current ConfigMap
	configMap, err := dm.clientset.CoreV1().ConfigMaps(dm.configMapNamespace).Get(ctx, dm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", dm.configMapNamespace, dm.configMapName, err)
	}

	// Generate new pipeline configuration
	pipelineConfig := dm.generateServicePipeline(req)

	// Update the otel-collector-config.yaml
	currentConfig := configMap.Data["otel-collector-config.yaml"]
	updatedConfig := dm.mergePipelineConfig(currentConfig, pipelineConfig, req.ServiceName)

	configMap.Data["otel-collector-config.yaml"] = updatedConfig

	// Update ConfigMap
	_, err = dm.clientset.CoreV1().ConfigMaps(dm.configMapNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	// Trigger DaemonSet reload
	return dm.triggerDaemonSetReload(ctx)
}

// RemoveServicePipeline removes a service pipeline from the DaemonSet configuration
func (dm *DaemonSetManager) RemoveServicePipeline(ctx context.Context, serviceName string) error {
	configMap, err := dm.clientset.CoreV1().ConfigMaps(dm.configMapNamespace).Get(ctx, dm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	currentConfig := configMap.Data["otel-collector-config.yaml"]
	updatedConfig := dm.removePipelineFromConfig(currentConfig, serviceName)

	configMap.Data["otel-collector-config.yaml"] = updatedConfig

	_, err = dm.clientset.CoreV1().ConfigMaps(dm.configMapNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return dm.triggerDaemonSetReload(ctx)
}

func (dm *DaemonSetManager) generateServicePipeline(req *models.ObservabilityRequest) string {
	// Generate service-specific pipeline configuration
	pipelineName := fmt.Sprintf("logs/%s", req.ServiceName)

	template := `
    %s:
      receivers: [otlp]
      processors:
        - resource/%s
        - batch
      exporters: [loki/%s]

  processors:
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
          action: upsert%s

  exporters:
    loki/%s:
      endpoint: "${LOKI_ENDPOINT}"
      labels:
        attributes:
          service_name: service.name
          namespace: service.namespace
          team: team
          level: level%s
      tenant_id: %s
`

	// Build custom labels
	customLabels := ""
	lokiLabels := ""
	for key, value := range req.CustomLabels {
		customLabels += fmt.Sprintf(`
        - key: %s
          value: %s
          action: upsert`, key, value)
		lokiLabels += fmt.Sprintf(`
          %s: %s`, key, key)
	}

	return fmt.Sprintf(template,
		pipelineName, req.ServiceName, req.ServiceName, // pipeline
		req.ServiceName, // processor
		req.ServiceName, req.Namespace, req.Team, customLabels, // resource attributes
		req.ServiceName, lokiLabels, req.Team) // exporter
}

func (dm *DaemonSetManager) mergePipelineConfig(currentConfig, newPipeline, serviceName string) string {
	// Simple merge logic - in production, you'd want to use a YAML parser
	// This is a basic string manipulation approach

	// Check if service pipeline already exists
	if strings.Contains(currentConfig, fmt.Sprintf("logs/%s:", serviceName)) {
		// Update existing pipeline
		return dm.replacePipelineInConfig(currentConfig, newPipeline, serviceName)
	}

	// Add new pipeline to service.pipelines section
	pipelinesIndex := strings.Index(currentConfig, "service:\n  pipelines:")
	if pipelinesIndex == -1 {
		return currentConfig + "\n" + newPipeline
	}

	// Insert new pipeline configuration
	before := currentConfig[:pipelinesIndex]
	after := currentConfig[pipelinesIndex:]

	return before + newPipeline + "\n" + after
}

func (dm *DaemonSetManager) replacePipelineInConfig(config, newPipeline, serviceName string) string {
	// Replace existing service pipeline - simplified implementation
	lines := strings.Split(config, "\n")
	var result []string
	inServiceSection := false

	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf("logs/%s:", serviceName)) {
			inServiceSection = true
			continue
		}
		if inServiceSection && strings.HasPrefix(line, "    logs/") {
			inServiceSection = false
		}
		if !inServiceSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n") + "\n" + newPipeline
}

func (dm *DaemonSetManager) removePipelineFromConfig(config, serviceName string) string {
	// Remove service pipeline from configuration
	lines := strings.Split(config, "\n")
	var result []string
	inServiceSection := false

	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf("logs/%s:", serviceName)) ||
		   strings.Contains(line, fmt.Sprintf("resource/%s:", serviceName)) ||
		   strings.Contains(line, fmt.Sprintf("loki/%s:", serviceName)) {
			inServiceSection = true
			continue
		}
		if inServiceSection && (strings.HasPrefix(line, "    logs/") ||
		   strings.HasPrefix(line, "  processors:") ||
		   strings.HasPrefix(line, "  exporters:")) {
			inServiceSection = false
		}
		if !inServiceSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func (dm *DaemonSetManager) triggerDaemonSetReload(ctx context.Context) error {
	// Get the DaemonSet
	daemonSet, err := dm.clientset.AppsV1().DaemonSets(dm.collectorNamespace).Get(ctx, "otel-collector", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get DaemonSet: %w", err)
	}

	// Add/update annotation to trigger pod restart
	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}

	daemonSet.Spec.Template.Annotations["config-reload-timestamp"] = metav1.Now().Format("2006-01-02T15:04:05Z")

	// Update DaemonSet
	_, err = dm.clientset.AppsV1().DaemonSets(dm.collectorNamespace).Update(ctx, daemonSet, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update DaemonSet: %w", err)
	}

	return nil
}