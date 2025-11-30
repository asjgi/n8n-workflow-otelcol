package otel

import (
	"context"
	"fmt"
	"strings"
	"otel-pipeline-automation/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CollectorRulesManager struct {
	clientset         kubernetes.Interface
	configMapName     string
	configMapNamespace string
}

func NewCollectorRulesManager(clientset kubernetes.Interface, configMapName, configMapNamespace string) *CollectorRulesManager {
	return &CollectorRulesManager{
		clientset:         clientset,
		configMapName:     configMapName,
		configMapNamespace: configMapNamespace,
	}
}

// AddServiceRule adds collection rule for a specific service
func (rm *CollectorRulesManager) AddServiceRule(ctx context.Context, req *models.ObservabilityRequest) error {
	configMap, err := rm.clientset.CoreV1().ConfigMaps(rm.configMapNamespace).Get(ctx, rm.configMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %s/%s: %w", rm.configMapNamespace, rm.configMapName, err)
	}

	// Generate filelog receiver for the service
	filelogReceiver := rm.generateFilelogReceiver(req)

	// Generate resource processor for labeling
	resourceProcessor := rm.generateResourceProcessor(req)

	// Generate routing processor for filtering
	routingProcessor := rm.generateRoutingProcessor(req)

	// Update configuration
	currentConfig := configMap.Data["otel-collector-config.yaml"]
	updatedConfig := rm.addServiceRuleToConfig(currentConfig, filelogReceiver, resourceProcessor, routingProcessor, req.ServiceName)

	configMap.Data["otel-collector-config.yaml"] = updatedConfig

	_, err = rm.clientset.CoreV1().ConfigMaps(rm.configMapNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return rm.triggerCollectorReload(ctx)
}

func (rm *CollectorRulesManager) generateFilelogReceiver(req *models.ObservabilityRequest) string {
	template := `
  filelog/%s:
    include:
      - /var/log/pods/%s_%s_*/*/%.log
    exclude:
      - /var/log/pods/%s_%s_*/*/*previous*.log
    start_at: end
    include_file_path: true
    include_file_name: false
    operators:
      - type: move
        from: attributes.log.file.path
        to: attributes.log_file_path
      - type: regex_parser
        regex: '^(?P<timestamp>\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z) (?P<level>\w+) (?P<message>.*)'
        timestamp:
          parse_from: timestamp
          layout: '2006-01-02T15:04:05.000Z'
      - type: move
        from: attributes.level
        to: attributes.log_level
`

	return fmt.Sprintf(template,
		req.ServiceName,
		req.Namespace, req.ServiceName, // include path
		req.Namespace, req.ServiceName) // exclude path
}

func (rm *CollectorRulesManager) generateResourceProcessor(req *models.ObservabilityRequest) string {
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
        value: %s
        action: upsert%s
`

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
		"${CLUSTER_NAME}", // will be replaced by env var
		customLabels)
}

func (rm *CollectorRulesManager) generateRoutingProcessor(req *models.ObservabilityRequest) string {
	template := `
  routing/%s:
    from_attribute: log_level
    default_pipeline: logs/%s_default
    table:
      - value: debug
        pipeline: logs/%s_debug
      - value: info
        pipeline: logs/%s_info
      - value: warn
        pipeline: logs/%s_warn
      - value: error
        pipeline: logs/%s_error
`

	return fmt.Sprintf(template,
		req.ServiceName,
		req.ServiceName,
		req.ServiceName,
		req.ServiceName,
		req.ServiceName,
		req.ServiceName)
}

func (rm *CollectorRulesManager) addServiceRuleToConfig(config, filelogReceiver, resourceProcessor, routingProcessor, serviceName string) string {
	// 이 함수는 YAML 파싱 라이브러리 사용을 권장하지만,
	// 간단한 구현을 위해 문자열 조작 사용

	// receivers 섹션에 추가
	receiversSection := "receivers:"
	config = addToSection(config, receiversSection, filelogReceiver)

	// processors 섹션에 추가
	processorsSection := "processors:"
	config = addToSection(config, processorsSection, resourceProcessor)
	config = addToSection(config, processorsSection, routingProcessor)

	// service.pipelines 섹션에 파이프라인 추가
	pipelinesConfig := rm.generateServicePipelines(serviceName)
	pipelinesSection := "service:\n  pipelines:"
	config = addToSection(config, pipelinesSection, pipelinesConfig)

	return config
}

func (rm *CollectorRulesManager) generateServicePipelines(serviceName string) string {
	template := `
    logs/%s_default:
      receivers: [filelog/%s]
      processors: [resource/%s, batch]
      exporters: [loki/%s]
    logs/%s_debug:
      receivers: [filelog/%s]
      processors: [resource/%s, batch]
      exporters: [loki/%s_debug]
    logs/%s_info:
      receivers: [filelog/%s]
      processors: [resource/%s, batch]
      exporters: [loki/%s]
    logs/%s_warn:
      receivers: [filelog/%s]
      processors: [resource/%s, batch]
      exporters: [loki/%s_alerts]
    logs/%s_error:
      receivers: [filelog/%s]
      processors: [resource/%s, batch]
      exporters: [loki/%s_alerts]
`

	return fmt.Sprintf(template,
		serviceName, serviceName, serviceName, serviceName, // default
		serviceName, serviceName, serviceName, serviceName, // debug
		serviceName, serviceName, serviceName, serviceName, // info
		serviceName, serviceName, serviceName, serviceName, // warn
		serviceName, serviceName, serviceName, serviceName) // error
}

// Helper function to add content to a YAML section
func addToSection(config, sectionHeader, newContent string) string {
	// 실제 구현에서는 yaml 라이브러리 사용 권장
	// 여기서는 간단한 문자열 조작으로 구현

	lines := strings.Split(config, "\n")
	var result []string
	found := false

	for i, line := range lines {
		result = append(result, line)
		if strings.Contains(line, sectionHeader) && !found {
			found = true
			// 다음 섹션 찾기
			nextSectionIndex := i + 1
			for j := i + 1; j < len(lines); j++ {
				if strings.HasPrefix(lines[j], "processors:") ||
				   strings.HasPrefix(lines[j], "exporters:") ||
				   strings.HasPrefix(lines[j], "service:") {
					nextSectionIndex = j
					break
				}
			}
			// 새 내용 삽입
			result = append(result, strings.Split(newContent, "\n")...)
		}
	}

	return strings.Join(result, "\n")
}

func (rm *CollectorRulesManager) triggerCollectorReload(ctx context.Context) error {
	// DaemonSet에 reload 신호 전송 (annotation 업데이트)
	daemonSet, err := rm.clientset.AppsV1().DaemonSets("observability").Get(ctx, "otel-collector", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get otel-collector DaemonSet: %w", err)
	}

	if daemonSet.Spec.Template.Annotations == nil {
		daemonSet.Spec.Template.Annotations = make(map[string]string)
	}

	daemonSet.Spec.Template.Annotations["config-reload-timestamp"] = metav1.Now().String()

	_, err = rm.clientset.AppsV1().DaemonSets("observability").Update(ctx, daemonSet, metav1.UpdateOptions{})
	return err
}