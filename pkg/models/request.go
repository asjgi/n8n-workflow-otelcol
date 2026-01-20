package models

// ObservabilityRequest represents the simplified request from Port IDP
type ObservabilityRequest struct {
	ServiceName  string            `json:"service_name" yaml:"service_name"`
	Namespace    string            `json:"namespace" yaml:"namespace"`
	Team         string            `json:"team" yaml:"team"`
	CustomLabels map[string]string `json:"custom_labels,omitempty" yaml:"custom_labels,omitempty"`
}

// PortWebhook represents the webhook payload from Port
type PortWebhook struct {
	Context struct {
		Entity     string `json:"entity"`
		Blueprint  string `json:"blueprint"`
		RunID      string `json:"runId"`
	} `json:"context"`
	Payload struct {
		Properties ObservabilityRequest `json:"properties"`
	} `json:"payload"`
	Action struct {
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
	} `json:"action"`
}

// N8nWebhookPayload represents the payload sent to n8n
type N8nWebhookPayload struct {
	RequestID string               `json:"request_id"`
	Action    string               `json:"action"`
	Request   ObservabilityRequest `json:"request"`
	Source    string               `json:"source"`
	Timestamp string               `json:"timestamp"`
}