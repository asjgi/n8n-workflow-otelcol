package models

import (
	"encoding/json"
	"testing"
)

func TestObservabilityRequest_JSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ObservabilityRequest
		wantErr  bool
	}{
		{
			name:  "basic request",
			input: `{"service_name":"my-service","namespace":"ns-test"}`,
			expected: ObservabilityRequest{
				ServiceName: "my-service",
				Namespace:   "ns-test",
			},
			wantErr: false,
		},
		{
			name:  "request with team",
			input: `{"service_name":"api-server","namespace":"production","team":"platform"}`,
			expected: ObservabilityRequest{
				ServiceName: "api-server",
				Namespace:   "production",
				Team:        "platform",
			},
			wantErr: false,
		},
		{
			name:  "request with custom labels",
			input: `{"service_name":"user-api","namespace":"staging","team":"backend","custom_labels":{"env":"stg","tier":"api"}}`,
			expected: ObservabilityRequest{
				ServiceName:  "user-api",
				Namespace:    "staging",
				Team:         "backend",
				CustomLabels: map[string]string{"env": "stg", "tier": "api"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ObservabilityRequest
			err := json.Unmarshal([]byte(tt.input), &got)

			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got.ServiceName != tt.expected.ServiceName {
				t.Errorf("ServiceName = %v, want %v", got.ServiceName, tt.expected.ServiceName)
			}
			if got.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace = %v, want %v", got.Namespace, tt.expected.Namespace)
			}
			if got.Team != tt.expected.Team {
				t.Errorf("Team = %v, want %v", got.Team, tt.expected.Team)
			}
			if len(got.CustomLabels) != len(tt.expected.CustomLabels) {
				t.Errorf("CustomLabels length = %v, want %v", len(got.CustomLabels), len(tt.expected.CustomLabels))
			}
		})
	}
}

func TestObservabilityRequest_Marshal(t *testing.T) {
	req := ObservabilityRequest{
		ServiceName:  "test-service",
		Namespace:    "test-ns",
		Team:         "devops",
		CustomLabels: map[string]string{"env": "test"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got ObservabilityRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.ServiceName != req.ServiceName {
		t.Errorf("ServiceName = %v, want %v", got.ServiceName, req.ServiceName)
	}
	if got.Team != req.Team {
		t.Errorf("Team = %v, want %v", got.Team, req.Team)
	}
}

func TestPortWebhook_JSON(t *testing.T) {
	input := `{
		"context": {
			"entity": "service-123",
			"blueprint": "observability-request",
			"runId": "run-456"
		},
		"payload": {
			"properties": {
				"service_name": "my-app",
				"namespace": "production"
			}
		},
		"action": {
			"identifier": "add-observability",
			"title": "Add Observability"
		}
	}`

	var webhook PortWebhook
	if err := json.Unmarshal([]byte(input), &webhook); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if webhook.Context.Entity != "service-123" {
		t.Errorf("Entity = %v, want service-123", webhook.Context.Entity)
	}
	if webhook.Context.Blueprint != "observability-request" {
		t.Errorf("Blueprint = %v, want observability-request", webhook.Context.Blueprint)
	}
	if webhook.Context.RunID != "run-456" {
		t.Errorf("RunID = %v, want run-456", webhook.Context.RunID)
	}
	if webhook.Payload.Properties.ServiceName != "my-app" {
		t.Errorf("ServiceName = %v, want my-app", webhook.Payload.Properties.ServiceName)
	}
	if webhook.Action.Identifier != "add-observability" {
		t.Errorf("Action.Identifier = %v, want add-observability", webhook.Action.Identifier)
	}
}

func TestN8nWebhookPayload_JSON(t *testing.T) {
	payload := N8nWebhookPayload{
		RequestID: "req-123",
		Action:    "create-pipeline",
		Request: ObservabilityRequest{
			ServiceName: "api",
			Namespace:   "default",
		},
		Source:    "port",
		Timestamp: "2024-01-20T10:00:00Z",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got N8nWebhookPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if got.RequestID != payload.RequestID {
		t.Errorf("RequestID = %v, want %v", got.RequestID, payload.RequestID)
	}
	if got.Source != "port" {
		t.Errorf("Source = %v, want port", got.Source)
	}
}
