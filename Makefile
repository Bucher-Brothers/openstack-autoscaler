# OpenStack Autoscaler Makefile

# Variables
BINARY_NAME=openstack-autoscaler
BINARY_PATH=./cmd
DOCKER_REGISTRY?=ghcr.io/bucher-brothers
IMAGE_NAME=$(DOCKER_REGISTRY)/openstack-autoscaler
VERSION?=latest

# Build targets
.PHONY: build clean test docker-build docker-push proto

# Build the binary
build:
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(BINARY_PATH)

# Build for Linux AMD64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 $(BINARY_PATH)

# Build for Linux ARM64
build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 $(BINARY_PATH)

# Clean build artifacts
clean:
	go clean
	rm -f $(BINARY_NAME)*

# Run tests
test:
	go test ./...

# Generate protobuf files
proto:
	protoc --go_out=api/protos --go_opt=paths=source_relative --go-grpc_out=api/protos --go-grpc_opt=paths=source_relative api/external-grpc.proto

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Download dependencies
deps:
	go mod download
	go mod tidy

# Docker build for AMD64
docker-build-amd64:
	docker build -f Dockerfile.amd64 -t $(IMAGE_NAME):$(VERSION)-amd64 .

# Docker build for ARM64
docker-build-arm64:
	docker build -f Dockerfile.arm64 -t $(IMAGE_NAME):$(VERSION)-arm64 .

# Docker build multi-arch
docker-build:
	docker buildx build --platform linux/amd64,linux/arm64 -f Dockerfile.amd64 -t $(IMAGE_NAME):$(VERSION) --push .

# Docker push
docker-push:
	docker push $(IMAGE_NAME):$(VERSION)-amd64
	docker push $(IMAGE_NAME):$(VERSION)-arm64

# Run locally
run:
	go run $(BINARY_PATH) --config=config.yaml

# Run with environment variables
run-env:
	go run $(BINARY_PATH) \
		--auth-url=$(OS_AUTH_URL) \
		--username=$(OS_USERNAME) \
		--password=$(OS_PASSWORD) \
		--project-name=$(OS_PROJECT_NAME) \
		--region=$(OS_REGION_NAME)

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  build-linux    - Build for Linux AMD64"
	@echo "  build-linux-arm64 - Build for Linux ARM64" 
	@echo "  clean          - Clean build artifacts"
	@echo "  test           - Run tests"
	@echo "  proto          - Generate protobuf files"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  deps           - Download dependencies"
	@echo "  docker-build-amd64 - Build Docker image for AMD64"
	@echo "  docker-build-arm64 - Build Docker image for ARM64"
	@echo "  docker-build   - Build multi-arch Docker image"
	@echo "  docker-push    - Push Docker images"
	@echo "  run            - Run locally with config file"
	@echo "  run-env        - Run locally with environment variables"
	@echo "  help           - Show this help"