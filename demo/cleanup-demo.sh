#!/bin/bash

echo "============================================"
echo "  Cleaning up demo resources..."
echo "============================================"

# Remove service from OTEL pipeline
echo "[1/3] Removing demo-order-service from OTEL pipeline..."
kubectl run curl-cleanup --rm -i --restart=Never --image=curlimages/curl -- \
  curl -s -X DELETE http://otel-pipeline-automation.ns-logging.svc.cluster.local:8080/api/v1/otel/pipeline/demo-order-service 2>/dev/null || true

# Delete demo namespace
echo "[2/3] Deleting ns-demo namespace..."
kubectl delete namespace ns-demo --ignore-not-found=true

# Also clean up test services
echo "[3/3] Cleaning up test services from OTEL pipeline..."
for svc in test-service n8n-test-service; do
  kubectl run curl-cleanup-$svc --rm -i --restart=Never --image=curlimages/curl -- \
    curl -s -X DELETE http://otel-pipeline-automation.ns-logging.svc.cluster.local:8080/api/v1/otel/pipeline/$svc 2>/dev/null || true
done

echo ""
echo "Cleanup complete!"
echo ""
echo "Current OTEL Collector receivers:"
kubectl get OpenTelemetryCollector my-collector -n ns-apigw -o jsonpath='{.spec.config.receivers}' | python3 -c "import sys,json; [print(f'  - {k}') for k in json.load(sys.stdin).keys()]" 2>/dev/null || echo "  (unable to retrieve)"
