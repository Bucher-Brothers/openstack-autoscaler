<div align="center">
  <img src="logo.png" alt="Bucher Brothers Logo" width="200"/>
  
  # OpenStack Autoscaler
  
  **A Kubernetes Cluster Autoscaler Provider for OpenStack**
  
  *Implementing the External gRPC Protocol for seamless OpenStack integration*
  
  [![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
  [![Go Report Card](https://goreportcard.com/badge/github.com/bucher-brothers/openstack-autoscaler)](https://goreportcard.com/report/github.com/bucher-brothers/openstack-autoscaler)
  [![Docker Pulls](https://img.shields.io/docker/pulls/bucherbrothers/openstack-autoscaler)](https://hub.docker.com/r/bucherbrothers/openstack-autoscaler)
  
</div>

---

## Overview

This OpenStack Autoscaler Provider enables the Kubernetes Cluster Autoscaler to automatically create and delete nodes in OpenStack environments. It implements the External gRPC Protocol and translates gRPC calls into OpenStack API calls using the gophercloud library.

## Features

- ✅ **External gRPC Protocol**: Complete implementation of the Kubernetes Cluster Autoscaler External gRPC Interface
- ✅ **OpenStack Integration**: Native integration via gophercloud v2 (latest and most efficient)
- ✅ **Multi-NodeGroup Support**: Support for multiple node groups with different configurations
- ✅ **Cloud-Init Support**: Flexible server initialization via Cloud-Init/User Data
- ✅ **TLS Support**: Secure gRPC communication with TLS
- ✅ **Multi-Architecture**: Support for AMD64 and ARM64
- ✅ **Container Ready**: Docker images available

## Quick Start with Helm

### Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- Access to an OpenStack environment
- `kubectl` configured for your cluster

### 1. Install OpenStack Autoscaler

```bash
# Clone the repository
git clone https://github.com/bucher-brothers/openstack-autoscaler
cd openstack-autoscaler

# Install via Helm with username/password authentication
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --create-namespace \
  --set openstack.auth.authUrl="https://keystone.example.com:5000/v3" \
  --set openstack.auth.region="RegionOne" \
  --set openstack.auth.username="your-username" \
  --set openstack.auth.password="your-password" \
  --set openstack.auth.projectName="your-project"

# OR install with OpenStack application credentials (recommended for production)
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --create-namespace \
  --set openstack.auth.authUrl="https://keystone.example.com:5000/v3" \
  --set openstack.auth.region="RegionOne" \
  --set openstack.auth.applicationCredentialId="your-app-cred-id" \
  --set openstack.auth.applicationCredentialSecret="your-app-cred-secret"
```

### 2. Install Kubernetes Cluster Autoscaler

```bash
# Add the autoscaler Helm repository
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm repo update

# Install cluster autoscaler configured for external-grpc
helm install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --set cloudProvider=external-grpc \
  --set extraArgs.cloud-provider-grpc-address="openstack-autoscaler.kube-system.svc.cluster.local:50051" \
  --set extraArgs.node-group-auto-discovery="openstack:name=worker-.*" \
  --set extraArgs.max-nodes-total=100 \
  --set extraArgs.scale-down-enabled=true \
  --set rbac.create=true
```

### 3. Verify Installation

```bash
# Check if both services are running
kubectl get pods -n kube-system -l app.kubernetes.io/name=openstack-autoscaler
kubectl get pods -n kube-system -l app.kubernetes.io/name=cluster-autoscaler

# Check logs
kubectl logs -n kube-system deployment/openstack-autoscaler
kubectl logs -n kube-system deployment/cluster-autoscaler
```

## 📖 Documentation & Configuration

For comprehensive configuration, installation options, troubleshooting, and advanced features, see the **[Helm Chart Documentation](./helm/openstack-autoscaler/README.md)**.

The Helm chart documentation includes:

- 🔧 **Complete Installation Guide** - Step-by-step setup with both autoscalers
- 🔐 **TLS Configuration** - Let's Encrypt integration with cert-manager
- ⚙️ **Advanced Configuration** - Production values, security, HA setup
- 🛠️ **Troubleshooting Guide** - Common issues and debugging steps
- 📊 **Architecture Details** - gRPC protocol and OpenStack integration

### Quick Configuration Examples

**Basic Setup:**

```bash
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --set openstack.auth.authUrl="https://keystone.example.com:5000/v3" \
  --set openstack.auth.username="your-username" \
  --set openstack.auth.password="your-password" \
  --set openstack.auth.projectName="your-project" \
  --set openstack.auth.region="RegionOne"
```

**Production with Existing Secret:**

```bash
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --set openstack.existingSecret="openstack-production-credentials"
```

## Alternative Installation Methods

<details>
<summary>🔧 Development & Testing (Non-Helm)</summary>

### Local Development

```bash
# Install dependencies
make deps

# Build binary
make build

# Run locally (requires OpenStack environment variables)
export OS_AUTH_URL="https://keystone.example.com:5000/v3"
export OS_USERNAME="your-username"
export OS_PASSWORD="your-password"
export OS_PROJECT_NAME="your-project"
export OS_REGION_NAME="RegionOne"
./openstack-autoscaler --v=4
```

### Docker Development

```bash
# Build Docker image
make docker-build-amd64

# Run with Docker
docker run -e OS_AUTH_URL="https://keystone.example.com:5000/v3" \
  -e OS_USERNAME="your-username" \
  -e OS_PASSWORD="your-password" \
  -e OS_PROJECT_NAME="your-project" \
  -e OS_REGION_NAME="RegionOne" \
  ghcr.io/bucher-brothers/openstack-autoscaler:latest
```

</details>

## Development Commands

```bash
# Generate Protobuf code
make proto

# Run tests
make test

make fmt

# Linting
make lint

# Helm development
helm lint ./helm/openstack-autoscaler
helm template ./helm/openstack-autoscaler --debug
```

## Project Structure

```
openstack-autoscaler/
├── cmd/                          # Main application
│   └── main.go                   # Entry point with CLI flags and server setup
├── pkg/                          # Public libraries
│   ├── config/                   # Configuration management
│   │   └── config.go             # YAML/Env configuration structures
│   ├── grpc/                     # gRPC Server implementation
│   │   └── grpc_server.go        # External gRPC Protocol server
│   └── provider/                 # OpenStack Provider core
│       ├── provider.go           # OpenStack client & management
│       └── nodegroup.go          # NodeGroup lifecycle management
├── internal/                     # Private helper libraries
│   └── utils/                    # Internal utilities
│       └── utils.go              # K8s Resource Quantity helpers
├── api/                          # API definitions
│   ├── external-grpc.proto       # Protobuf schema
│   └── protos/                   # Generated Go files
│       ├── external-grpc.pb.go
│       └── external-grpc_grpc.pb.go
├── Dockerfile.amd64              # Multi-arch container images
├── Dockerfile.arm64
├── Makefile                      # Build & development tasks
├── helm/                         # Helm Chart for deployment
│   └── openstack-autoscaler/     # Production-ready Helm chart
├── config.yaml.example           # Example configuration (dev only)
├── go.mod                        # Go module definition
└── README.md                     # Project documentation
```

## Troubleshooting

### Common Issues

**1. gRPC Connection Failed**

```bash
# Check if OpenStack Autoscaler is running
kubectl get pods -n kube-system -l app.kubernetes.io/name=openstack-autoscaler

# Test gRPC connectivity
kubectl exec -n kube-system deployment/cluster-autoscaler -- \
  nc -z openstack-autoscaler.kube-system.svc.cluster.local 50051
```

**2. OpenStack Authentication Issues**

```bash
# Check OpenStack credentials
kubectl get secret -n kube-system openstack-autoscaler-openstack -o yaml

# View autoscaler logs
kubectl logs -n kube-system deployment/openstack-autoscaler -f
```

**3. Cluster Autoscaler Not Scaling**

```bash
# Check cluster autoscaler logs
kubectl logs -n kube-system deployment/cluster-autoscaler -f

# Verify node group discovery
kubectl logs -n kube-system deployment/cluster-autoscaler | grep "node group"
```

### Helm Commands

```bash
# List releases
helm list -n kube-system

# Upgrade releases
helm upgrade openstack-autoscaler ./helm/openstack-autoscaler -n kube-system

# Uninstall
helm uninstall openstack-autoscaler -n kube-system
helm uninstall cluster-autoscaler -n kube-system

# Debug rendering
helm template ./helm/openstack-autoscaler --values your-values.yaml
```

## Architecture

```
┌─────────────────────────┐    gRPC     ┌──────────────────────────┐
│   Cluster Autoscaler    │◄──────────►│  OpenStack Autoscaler    │
│                         │             │                          │
│  - Scale Up/Down Logic  │             │  - gRPC Server           │
│  - Node Management      │             │  - OpenStack Provider    │
│  - External gRPC Client │             │  - Node Groups           │
└─────────────────────────┘             └──────────────────────────┘
                                                    │
                                                    │ gophercloud
                                                    ▼
                                        ┌──────────────────────────┐
                                        │     OpenStack APIs       │
                                        │                          │
                                        │  - Nova (Compute)        │
                                        │  - Glance (Images)       │
                                        │  - Neutron (Network)     │
                                        │  - Keystone (Identity)   │
                                        └──────────────────────────┘
```

---

<div align="center">

## 🤝 Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

If you encounter any problems or have questions, please create an issue in the GitHub repository.

## 🏢 About Bucher Brothers

Bucher Brothers is a technology consulting company specializing in cloud-native solutions, Kubernetes, and DevOps practices.

**Made with ❤️ by [Bucher Brothers](https://github.com/Bucher-Brothers)**

</div>
