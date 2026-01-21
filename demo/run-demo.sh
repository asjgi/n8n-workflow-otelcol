#!/bin/bash

set -e

echo "============================================"
echo "  OTEL Pipeline Automation Demo"
echo "============================================"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Step 1: Deploy demo application
echo -e "${YELLOW}[Step 1]${NC} Deploying demo-order-service..."
kubectl apply -f "$SCRIPT_DIR/demo-app.yaml"
echo -e "${GREEN}✓${NC} Demo app deployed"
echo ""

# Wait for pod to be ready
echo -e "${YELLOW}[Step 2]${NC} Waiting for pod to be ready..."
kubectl wait --for=condition=ready pod -l app=demo-order-service -n ns-demo --timeout=60s
echo -e "${GREEN}✓${NC} Pod is running"
echo ""

# Show current logs from the app
echo -e "${YELLOW}[Step 3]${NC} Sample logs from demo-order-service:"
echo "-------------------------------------------"
kubectl logs -n ns-demo -l app=demo-order-service --tail=3
echo "-------------------------------------------"
echo ""

# Step 4: Check current OTEL receivers
echo -e "${YELLOW}[Step 4]${NC} Current OTEL Collector receivers:"
kubectl get OpenTelemetryCollector my-collector -n ns-apigw -o jsonpath='{.spec.config.receivers}' | python3 -c "import sys,json; [print(f'  - {k}') for k in json.load(sys.stdin).keys()]"
echo ""

# Step 5: Add the new service via API
echo -e "${YELLOW}[Step 5]${NC} Adding demo-order-service to OTEL pipeline..."
echo ""
echo "Request:"
echo '  POST /api/v1/otel/pipeline/add'
echo '  {"service_name":"demo-order-service","namespace":"ns-demo","team":"demo-team"}'
echo ""

RESPONSE=$(kubectl exec -n ns-logging deploy/otel-pipeline-automation -- \
  wget -qO- --post-data='{"service_name":"demo-order-service","namespace":"ns-demo","team":"demo-team"}' \
  --header='Content-Type: application/json' \
  http://localhost:8080/api/v1/otel/pipeline/add)

echo "Response:"
echo "$RESPONSE" | python3 -m json.tool
echo ""
echo -e "${GREEN}✓${NC} Service added to pipeline"
echo ""

# Step 6: Verify receiver was added
echo -e "${YELLOW}[Step 6]${NC} Updated OTEL Collector receivers:"
kubectl get OpenTelemetryCollector my-collector -n ns-apigw -o jsonpath='{.spec.config.receivers}' | python3 -c "import sys,json; [print(f'  - {k}') for k in json.load(sys.stdin).keys()]"
echo ""

# Step 7: Wait for collector to restart
echo -e "${YELLOW}[Step 7]${NC} Waiting for OTEL Collector to restart..."
sleep 5
kubectl get pods -n ns-apigw -l app.kubernetes.io/name=my-collector-collector
echo ""

# Step 8: Check Loki for new service
echo -e "${YELLOW}[Step 8]${NC} Waiting for logs to appear in Loki (30s)..."
sleep 30

echo "Checking Loki for service_name labels:"
kubectl exec -n ns-observability deploy/loki-grafana -- \
  wget -qO- --header="X-Scope-OrgID: kic-local-tcn-loki" \
  'http://loki-gateway.ns-loki.svc.cluster.local:80/loki/api/v1/label/service_name/values' 2>/dev/null | \
  python3 -c "import sys,json; d=json.load(sys.stdin); print('Services in Loki:'); [print(f'  - {s}') for s in d.get('data',[])]"

echo ""
echo "============================================"
echo -e "${GREEN}  Demo Complete!${NC}"
echo "============================================"
echo ""
echo "Next steps:"
echo "  1. Open Grafana: kubectl port-forward -n ns-observability svc/loki-grafana 3000:80"
echo "  2. Go to http://localhost:3000"
echo "  3. Navigate to Explore > Loki"
echo "  4. Query: {service_name=\"demo-order-service\"}"
echo ""
