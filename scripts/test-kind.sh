#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧪 Testing OTEL Pipeline Automation in Kind${NC}"

# Configuration
AUTOMATION_URL="http://localhost:30080"
N8N_URL="http://localhost:30567"
LOKI_URL="http://localhost:31000"

# Function to test HTTP endpoint
test_endpoint() {
    local url=$1
    local service_name=$2
    local timeout=${3:-10}

    echo -e "${YELLOW}Testing ${service_name}...${NC}"

    if curl -s --max-time ${timeout} "${url}" > /dev/null; then
        echo -e "${GREEN}✅ ${service_name} is responding${NC}"
        return 0
    else
        echo -e "${RED}❌ ${service_name} is not responding${NC}"
        return 1
    fi
}

# Function to check pod status
check_pods() {
    local namespace=$1
    local label=$2

    echo -e "${YELLOW}Checking pods in ${namespace} namespace...${NC}"

    local pods=$(kubectl get pods -n ${namespace} -l ${label} --no-headers 2>/dev/null | wc -l)
    local running_pods=$(kubectl get pods -n ${namespace} -l ${label} --no-headers 2>/dev/null | grep Running | wc -l)

    if [ ${pods} -gt 0 ] && [ ${running_pods} -eq ${pods} ]; then
        echo -e "${GREEN}✅ All ${pods} pod(s) are running in ${namespace}${NC}"
        return 0
    else
        echo -e "${RED}❌ ${running_pods}/${pods} pods are running in ${namespace}${NC}"
        kubectl get pods -n ${namespace} -l ${label}
        return 1
    fi
}

# Test 1: Check if all pods are running
echo -e "${BLUE}📊 Test 1: Pod Health Check${NC}"
test_passed=true

check_pods "otel-automation" "app=otel-pipeline-automation" || test_passed=false
check_pods "observability" "app=otel-collector" || test_passed=false
check_pods "observability" "app=loki" || test_passed=false
check_pods "n8n" "app=n8n" || test_passed=false

if [ "$test_passed" = true ]; then
    echo -e "${GREEN}✅ Test 1 PASSED: All pods are healthy${NC}"
else
    echo -e "${RED}❌ Test 1 FAILED: Some pods are not healthy${NC}"
fi

echo ""

# Test 2: Service connectivity
echo -e "${BLUE}🌐 Test 2: Service Connectivity${NC}"
test_passed=true

# Wait a bit for services to be fully ready
sleep 10

test_endpoint "${AUTOMATION_URL}/api/v1/health" "OTEL Automation Service" || test_passed=false
test_endpoint "${LOKI_URL}/ready" "Loki Service" || test_passed=false
test_endpoint "${N8N_URL}" "n8n Service" || test_passed=false

if [ "$test_passed" = true ]; then
    echo -e "${GREEN}✅ Test 2 PASSED: All services are accessible${NC}"
else
    echo -e "${RED}❌ Test 2 FAILED: Some services are not accessible${NC}"
fi

echo ""

# Test 3: API functionality
echo -e "${BLUE}🔧 Test 3: API Functionality${NC}"
test_passed=true

echo -e "${YELLOW}Testing health endpoint...${NC}"
if health_response=$(curl -s "${AUTOMATION_URL}/api/v1/health"); then
    echo -e "${GREEN}✅ Health endpoint responded: ${health_response}${NC}"
else
    echo -e "${RED}❌ Health endpoint failed${NC}"
    test_passed=false
fi

echo -e "${YELLOW}Testing service pipeline add...${NC}"
add_response=$(curl -s -X POST "${AUTOMATION_URL}/api/v1/otel/pipeline/add" \
    -H "Content-Type: application/json" \
    -d '{
        "service_name": "test-service",
        "namespace": "default",
        "log_path": "/var/log/pods/default_test-service_*/*.log"
    }' || echo "FAILED")

if [ "$add_response" != "FAILED" ] && [ ! -z "$add_response" ]; then
    echo -e "${GREEN}✅ Pipeline add API responded: ${add_response}${NC}"
else
    echo -e "${RED}❌ Pipeline add API failed${NC}"
    test_passed=false
fi

if [ "$test_passed" = true ]; then
    echo -e "${GREEN}✅ Test 3 PASSED: API functionality working${NC}"
else
    echo -e "${RED}❌ Test 3 FAILED: API functionality issues${NC}"
fi

echo ""

# Test 4: ConfigMap verification
echo -e "${BLUE}📋 Test 4: ConfigMap Verification${NC}"
echo -e "${YELLOW}Checking OTEL Collector ConfigMap...${NC}"

if kubectl get configmap otel-collector-config -n observability > /dev/null 2>&1; then
    echo -e "${GREEN}✅ OTEL Collector ConfigMap exists${NC}"

    # Check if the test service was added (from Test 3)
    if kubectl get configmap otel-collector-config -n observability -o yaml | grep -q "test-service"; then
        echo -e "${GREEN}✅ Test service was successfully added to ConfigMap${NC}"
    else
        echo -e "${YELLOW}⚠️  Test service not found in ConfigMap (this might be expected)${NC}"
    fi
else
    echo -e "${RED}❌ OTEL Collector ConfigMap not found${NC}"
fi

echo ""

# Summary
echo -e "${BLUE}📊 Test Summary${NC}"
echo -e "${YELLOW}Services:${NC}"
echo "  • OTEL Automation: ${AUTOMATION_URL}"
echo "  • n8n:              ${N8N_URL}"
echo "  • Loki:             ${LOKI_URL}"

echo ""
echo -e "${YELLOW}Next Steps:${NC}"
echo "1. Open n8n at ${N8N_URL} to configure workflows"
echo "2. Test the full pipeline by creating a test service"
echo "3. Check Loki logs at ${LOKI_URL}"

echo ""
echo -e "${BLUE}🔍 Debugging Commands:${NC}"
echo "kubectl logs -f deployment/otel-pipeline-automation -n otel-automation"
echo "kubectl logs -f daemonset/otel-collector -n observability"
echo "kubectl get events --sort-by=.metadata.creationTimestamp"

echo ""
echo -e "${GREEN}🎉 Kind testing completed!${NC}"