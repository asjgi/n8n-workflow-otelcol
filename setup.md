# OTEL-Loki Pipeline Automation with Port IDP

## Architecture Overview

```
Port IDP → Port Action → n8n Webhook → OTEL Collector CRD → Kubernetes → Loki
```

## Components

1. **Port Blueprint**: Service observability request form
2. **n8n Workflow**: Automation pipeline
3. **Config Generator**: OpenTelemetry Collector CRD creator
4. **Kubernetes Integration**: Apply configurations via kubectl

## Technology Stack

- Port IDP for developer self-service
- n8n for workflow automation
- OpenTelemetry Operator 0.123.0
- Loki for log aggregation
- Python/Node.js for config generation

## Next Steps

1. Create Port blueprint
2. Setup n8n workflow
3. Implement config generator
4. Test end-to-end flow