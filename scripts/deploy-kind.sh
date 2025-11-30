#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
IMAGE_NAME="otel-pipeline-automation"
IMAGE_TAG="local"
FULL_IMAGE_NAME="${IMAGE_NAME}:${IMAGE_TAG}"

echo -e "${BLUE}🚀 Starting Kind deployment for OTEL Pipeline Automation${NC}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
echo -e "${YELLOW}📋 Checking prerequisites...${NC}"
if ! command_exists docker; then
    echo -e "${RED}❌ Docker is not installed${NC}"
    exit 1
fi

if ! command_exists kind; then
    echo -e "${RED}❌ Kind is not installed${NC}"
    exit 1
fi

if ! command_exists kubectl; then
    echo -e "${RED}❌ kubectl is not installed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Prerequisites check passed${NC}"

# Check if Kind cluster exists
echo -e "${YELLOW}🔍 Checking Kind cluster status...${NC}"
if ! kind get clusters | grep -q "kind"; then
    echo -e "${RED}❌ No Kind cluster found. Please create a Kind cluster first:${NC}"
    echo -e "${BLUE}kind create cluster${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Kind cluster found${NC}"

# Build Docker image
echo -e "${YELLOW}🏗️  Building Docker image...${NC}"
if ! docker build -t ${FULL_IMAGE_NAME} .; then
    echo -e "${RED}❌ Docker build failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Docker image built: ${FULL_IMAGE_NAME}${NC}"

# Load image into Kind cluster
echo -e "${YELLOW}📦 Loading image into Kind cluster...${NC}"
if ! kind load docker-image ${FULL_IMAGE_NAME}; then
    echo -e "${RED}❌ Failed to load image into Kind cluster${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Image loaded into Kind cluster${NC}"

# Apply Kubernetes manifests
echo -e "${YELLOW}🚀 Deploying to Kubernetes...${NC}"
if ! kubectl apply -f deployments/kind-local.yaml; then
    echo -e "${RED}❌ Kubernetes deployment failed${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Kubernetes manifests applied${NC}"

# Wait for deployments to be ready
echo -e "${YELLOW}⏳ Waiting for deployments to be ready...${NC}"

# Wait for n8n
echo -e "${BLUE}Waiting for n8n...${NC}"
kubectl wait --for=condition=available --timeout=300s deployment/n8n -n n8n || true

# Wait for loki
echo -e "${BLUE}Waiting for Loki...${NC}"
kubectl wait --for=condition=available --timeout=300s deployment/loki -n observability || true

# Wait for otel-collector daemonset
echo -e "${BLUE}Waiting for OTEL Collector...${NC}"
kubectl rollout status daemonset/otel-collector -n observability --timeout=300s || true

# Wait for automation service
echo -e "${BLUE}Waiting for automation service...${NC}"
kubectl wait --for=condition=available --timeout=300s deployment/otel-pipeline-automation -n otel-automation || true

echo -e "${GREEN}✅ All deployments are ready${NC}"

# Display service information
echo -e "${BLUE}📊 Service Information:${NC}"
echo -e "${YELLOW}n8n:${NC} http://localhost:30567"
echo -e "${YELLOW}Loki:${NC} http://localhost:31000"
echo -e "${YELLOW}OTEL Automation:${NC} http://localhost:30080"

echo ""
echo -e "${BLUE}🔍 Check deployment status:${NC}"
echo "kubectl get pods -n otel-automation"
echo "kubectl get pods -n observability"
echo "kubectl get pods -n n8n"

echo ""
echo -e "${BLUE}📋 Test API endpoints:${NC}"
echo "curl http://localhost:30080/api/v1/health"

echo ""
echo -e "${GREEN}🎉 Kind deployment completed successfully!${NC}"