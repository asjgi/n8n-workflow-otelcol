#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧹 Cleaning up Kind deployment for OTEL Pipeline Automation${NC}"

# Delete Kubernetes resources
echo -e "${YELLOW}🗑️  Deleting Kubernetes resources...${NC}"

# Delete in reverse order to avoid dependency issues
kubectl delete -f deployments/kind-local.yaml --ignore-not-found=true || true

# Wait a moment for cleanup
sleep 5

# Check if resources are deleted
echo -e "${YELLOW}🔍 Checking cleanup status...${NC}"

echo -e "${BLUE}Checking namespaces...${NC}"
kubectl get namespaces | grep -E "(otel-automation|observability|n8n)" || echo -e "${GREEN}✅ All namespaces cleaned up${NC}"

# Remove docker image from Kind cluster
echo -e "${YELLOW}🐳 Removing Docker image from local registry...${NC}"
docker rmi otel-pipeline-automation:local --force 2>/dev/null || echo -e "${BLUE}Image already removed or not found${NC}"

echo -e "${GREEN}🎉 Cleanup completed!${NC}"
echo -e "${BLUE}To completely reset Kind cluster, run:${NC}"
echo -e "${YELLOW}kind delete cluster${NC}"