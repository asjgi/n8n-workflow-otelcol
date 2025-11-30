.PHONY: build run test clean docker-build docker-push deploy

# Variables
IMAGE_NAME := otel-pipeline-automation
IMAGE_TAG := latest
REGISTRY := your-registry.com

# Go commands
build:
	go build -o bin/otel-automation ./cmd/main.go

run:
	go run ./cmd/main.go

test:
	go test ./...

clean:
	rm -rf bin/

# Docker commands
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-push:
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Kubernetes commands
deploy:
	kubectl apply -f deployments/kubernetes.yaml

undeploy:
	kubectl delete -f deployments/kubernetes.yaml

# Port setup
setup-port:
	@echo "Please create the Port blueprint using the JSON in port-blueprint.json"
	@echo "Then create a Port action with webhook URL pointing to your service"

# n8n setup
setup-n8n:
	@echo "Import the n8n workflow from configs/n8n-workflow.json"
	@echo "Configure Port API token credential in n8n"

# Development
dev-setup: setup-port setup-n8n
	@echo "Development environment setup complete"

# Complete deployment
full-deploy: docker-build deploy
	@echo "Full deployment complete"

help:
	@echo "Available commands:"
	@echo "  build        - Build the Go binary"
	@echo "  run          - Run the application locally"
	@echo "  test         - Run tests"
	@echo "  docker-build - Build Docker image"
	@echo "  deploy       - Deploy to Kubernetes"
	@echo "  setup-port   - Instructions for Port setup"
	@echo "  setup-n8n    - Instructions for n8n setup"
	@echo "  full-deploy  - Build and deploy everything"