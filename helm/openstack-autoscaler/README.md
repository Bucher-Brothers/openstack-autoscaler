# OpenStack Autoscaler Helm Chart

This Helm chart deploys the OpenStack Autoscaler Provider for the Kubernetes Cluster Autoscaler, implementing the External gRPC Protocol.

## Overview

The OpenStack Autoscaler Provider enables the Kubernetes Cluster Autoscaler to automatically create and delete nodes in OpenStack environments. It implements the External gRPC Protocol and translates gRPC calls into OpenStack API calls using the gophercloud v2 library.

### Features

- ✅ **External gRPC Protocol**: Complete implementation of the Kubernetes Cluster Autoscaler External gRPC Interface
- ✅ **OpenStack Integration**: Native integration via gophercloud v2 (latest and most efficient)
- ✅ **Multi-NodeGroup Support**: Support for multiple node groups with different configurations
- ✅ **Cloud-Init Support**: Flexible server initialization via Cloud-Init/User Data
- ✅ **TLS Support**: Secure gRPC communication with TLS (including Let's Encrypt)
- ✅ **Multi-Architecture**: Support for AMD64 and ARM64
- ✅ **Production Ready**: Helm chart with security best practices

### Architecture

```
┌─────────────────────────┐    gRPC     ┌──────────────────────────┐
│   Cluster Autoscaler    │◄──────────►│  OpenStack Autoscaler    │
│                         │             │                          │
│  - Scale Up/Down Logic  │             │  - gRPC Server           │
│  - Node Management      │             │  - OpenStack Provider    │
│  - External gRPC Client │             │  - Node Groups           │
└─────────────────────────┘             └──────────────────────────┘
                                                    │
                                                    │ gophercloud v2
                                                    ▼
                                        ┌──────────────────────────┐
                                        │     OpenStack APIs       │
                                        │  - Nova (Compute)        │
                                        │  - Glance (Images)       │
                                        │  - Neutron (Networking)  │
                                        │  - Keystone (Identity)   │
                                        └──────────────────────────┘
```

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- Access to an OpenStack environment
- `kubectl` configured for your cluster
- Optional: cert-manager for Let's Encrypt TLS certificates

## Quick Start

### 1. Install OpenStack Autoscaler

```bash
# Clone the repository
git clone https://github.com/bucher-brothers/openstack-autoscaler
cd openstack-autoscaler

# Install via Helm with your OpenStack credentials
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --create-namespace \
  --set openstack.auth.authUrl="https://keystone.example.com:5000/v3" \
  --set openstack.auth.username="your-username" \
  --set openstack.auth.password="your-password" \
  --set openstack.auth.projectName="your-project" \
  --set openstack.auth.region="RegionOne"
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

# Test gRPC connectivity
kubectl exec -n kube-system deployment/cluster-autoscaler -- \
  nc -z openstack-autoscaler.kube-system.svc.cluster.local 50051

# Check logs
kubectl logs -n kube-system deployment/openstack-autoscaler -f
kubectl logs -n kube-system deployment/cluster-autoscaler -f
```

## Configuration

### OpenStack Credentials

You can configure OpenStack credentials in several ways:

#### Method 1: Direct Values

```yaml
openstack:
  auth:
    authUrl: "https://keystone.example.com:5000/v3"
    username: "your-username"
    password: "your-password"
    projectName: "your-project"
    region: "RegionOne"
    userDomainName: "Default"
    projectDomainName: "Default"
    interface: "public"
```

#### Method 2: Existing Secret

```yaml
openstack:
  existingSecret: "my-openstack-credentials"
  secretKeys:
    authUrl: "OS_AUTH_URL"
    username: "OS_USERNAME"
    password: "OS_PASSWORD"
    projectName: "OS_PROJECT_NAME"
    region: "OS_REGION_NAME"
```

Create the secret manually:

```bash
kubectl create secret generic my-openstack-credentials \
  --from-literal=OS_AUTH_URL="https://keystone.example.com:5000/v3" \
  --from-literal=OS_USERNAME="your-username" \
  --from-literal=OS_PASSWORD="your-password" \
  --from-literal=OS_PROJECT_NAME="your-project" \
  --from-literal=OS_REGION_NAME="RegionOne" \
  --from-literal=OS_USER_DOMAIN_NAME="Default" \
  --from-literal=OS_PROJECT_DOMAIN_NAME="Default" \
  --from-literal=OS_INTERFACE="public" \
  --namespace kube-system
```

## TLS Configuration with Let's Encrypt

### Prerequisites for TLS

```bash
# Install cert-manager if not already installed
helm repo add jetstack https://charts.jetstack.io
helm repo update

helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --set installCRDs=true
```

### Option 1: Let's Encrypt with HTTP01 Challenge

1. **Create ClusterIssuer**:

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: nginx
```

2. **Enable TLS in Helm values**:

```yaml
grpc:
  tls:
    enabled: true
    cert: "/certs/tls.crt"
    key: "/certs/tls.key"

# Certificate configuration
certificate:
  enabled: true
  issuer: letsencrypt-prod
  dnsNames:
    - openstack-autoscaler.your-domain.com

# Mount TLS certificates
volumes:
  - name: tls-certs
    secret:
      secretName: openstack-autoscaler-tls

volumeMounts:
  - name: tls-certs
    mountPath: /certs
    readOnly: true
```

### Option 2: Let's Encrypt with DNS01 Challenge

1. **Create ClusterIssuer with DNS provider** (example with Cloudflare):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-dns-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-dns-prod
    solvers:
      - dns01:
          cloudflare:
            email: your-email@example.com
            apiTokenSecretRef:
              name: cloudflare-api-token-secret
              key: api-token
```

2. **Create Cloudflare API token secret**:

```bash
kubectl create secret generic cloudflare-api-token-secret \
  --from-literal=api-token=your-cloudflare-api-token \
  --namespace kube-system
```

3. **Configure Helm values for DNS01**:

```yaml
grpc:
  tls:
    enabled: true
    cert: "/certs/tls.crt"
    key: "/certs/tls.key"

certificate:
  enabled: true
  issuer: letsencrypt-dns-prod
  dnsNames:
    - openstack-autoscaler.your-domain.com
  dnsNames:
    - "*.your-domain.com"  # wildcard support with DNS01

volumes:
  - name: tls-certs
    secret:
      secretName: openstack-autoscaler-tls

volumeMounts:
  - name: tls-certs
    mountPath: /certs
    readOnly: true
```

### Manual TLS Certificate

For existing certificates:

```yaml
grpc:
  tls:
    enabled: true
    cert: "/certs/tls.crt"
    key: "/certs/tls.key"
    ca: "/certs/ca.crt"

volumes:
  - name: tls-certs
    secret:
      secretName: your-existing-tls-secret

volumeMounts:
  - name: tls-certs
    mountPath: /certs
    readOnly: true
```

## Configuration

### Basic Helm Values

```yaml
# Basic OpenStack configuration
openstack:
  auth:
    authUrl: "https://keystone.example.com:5000/v3"
    username: "your-username"
    password: "your-password"
    projectName: "your-project"
    region: "RegionOne"
    userDomainName: "Default"
    projectDomainName: "Default"
    interface: "public"

# Resource limits
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

# gRPC server configuration
grpc:
  address: ":50051"
  tls:
    enabled: false
```

### Using Existing Secrets (Recommended for Production)

For production environments, use existing Kubernetes secrets instead of plain text values:

```yaml
openstack:
  existingSecret: "openstack-production-credentials"
  secretKeys:
    authUrl: "OS_AUTH_URL"
    username: "OS_USERNAME"
    password: "OS_PASSWORD"
    projectName: "OS_PROJECT_NAME"
    region: "OS_REGION_NAME"
```

Create the secret:

```bash
kubectl create secret generic openstack-production-credentials \
  --from-literal=OS_AUTH_URL="https://keystone.example.com:5000/v3" \
  --from-literal=OS_USERNAME="your-username" \
  --from-literal=OS_PASSWORD="your-password" \
  --from-literal=OS_PROJECT_NAME="your-project" \
  --from-literal=OS_REGION_NAME="RegionOne" \
  --from-literal=OS_USER_DOMAIN_NAME="Default" \
  --from-literal=OS_PROJECT_DOMAIN_NAME="Default" \
  --from-literal=OS_INTERFACE="public" \
  --namespace kube-system
```

### Node Group Configuration

**Important**: Node groups are dynamically managed by the Kubernetes Cluster Autoscaler through the external-grpc protocol. The Cluster Autoscaler will:

1. Send node group definitions via gRPC calls
2. Request scaling operations (increase/decrease nodes)
3. Provide all necessary OpenStack configuration (flavor, image, network, etc.)

This service only needs **OpenStack cloud credentials** - no static node group configuration required.

### High Availability Configuration

```yaml
# Multiple replicas with pod disruption budget
replicaCount: 2

podDisruptionBudget:
  enabled: true
  minAvailable: 1

# Anti-affinity to spread pods across nodes
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app.kubernetes.io/name
            operator: In
            values:
            - openstack-autoscaler
        topologyKey: kubernetes.io/hostname

# Resource configuration for production
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - openstack-autoscaler
          topologyKey: kubernetes.io/hostname
```

### Security

```yaml
# Network Policies
networkPolicy:
  enabled: true
  ingress:
    # Allow from cluster autoscaler namespace
    - from:
        - namespaceSelector:
            matchLabels:
              name: cluster-autoscaler
      ports:
        - protocol: TCP
          port: 50051

# Security Context
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65534
```

## Integration with Cluster Autoscaler

### Complete Setup via Helm Charts

#### Step 1: Install OpenStack Autoscaler

```bash
# Install the OpenStack Autoscaler Provider
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --set openstack.auth.authUrl="https://keystone.example.com:5000/v3" \
  --set openstack.auth.username="your-username" \
  --set openstack.auth.password="your-password" \
  --set openstack.auth.projectName="your-project" \
  --set openstack.auth.region="RegionOne"
```

#### Step 2: Install Cluster Autoscaler

```bash
# Add the autoscaler helm repository
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

### Alternative: Using Values Files

#### 1. OpenStack Autoscaler Values (openstack-autoscaler-values.yaml)

```yaml
openstack:
  auth:
    authUrl: "https://keystone.example.com:5000/v3"
    username: "your-username"
    password: "your-password"
    projectName: "your-project"
    region: "RegionOne"
    userDomainName: "Default"
    projectDomainName: "Default"
    interface: "public"

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

nodeSelector:
  kubernetes.io/os: linux
```

#### 2. Cluster Autoscaler Values (cluster-autoscaler-values.yaml)

```yaml
cloudProvider: external-grpc

extraArgs:
  cloud-provider-grpc-address: "openstack-autoscaler.kube-system.svc.cluster.local:50051"
  node-group-auto-discovery: "openstack:name=worker-.*"
  max-nodes-total: 100
  cores-total: "0:320"
  memory-total: "0:1280"
  scale-down-enabled: true
  scale-down-delay-after-add: "10m"
  scale-down-unneeded-time: "10m"
  skip-nodes-with-local-storage: false
  skip-nodes-with-system-pods: false
  v: 2

replicaCount: 1

rbac:
  create: true

resources:
  limits:
    cpu: 100m
    memory: 300Mi
  requests:
    cpu: 100m
    memory: 300Mi

nodeSelector:
  kubernetes.io/os: linux

tolerations:
  - effect: NoSchedule
    operator: Equal
    key: node-role.kubernetes.io/control-plane
  - effect: NoSchedule
    operator: Equal
    key: node-role.kubernetes.io/master
```

#### 3. Deploy with Values Files

```bash
# Install OpenStack Autoscaler
helm install openstack-autoscaler ./helm/openstack-autoscaler \
  --namespace kube-system \
  --values openstack-autoscaler-values.yaml

# Install Cluster Autoscaler
helm install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --values cluster-autoscaler-values.yaml
```

### Verification

````bash
# Check both services are running
kubectl get pods -n kube-system -l app.kubernetes.io/name=openstack-autoscaler
kubectl get pods -n kube-system -l app.kubernetes.io/name=cluster-autoscaler

# Check logs
kubectl logs -n kube-system deployment/openstack-autoscaler
kubectl logs -n kube-system deployment/cluster-autoscaler

# Test gRPC connectivity
kubectl exec -n kube-system deployment/cluster-autoscaler -- \
  nc -z openstack-autoscaler.kube-system.svc.cluster.local 50051
```## Values Reference

| Parameter                    | Description                                | Default                                        |
| ---------------------------- | ------------------------------------------ | ---------------------------------------------- |
| `image.repository`           | Container image repository                 | `ghcr.io/bucher-brothers/openstack-autoscaler` |
| `image.tag`                  | Container image tag                        | `""` (uses appVersion)                         |
| `image.pullPolicy`           | Container image pull policy                | `IfNotPresent`                                 |
| `serviceAccount.create`      | Create service account                     | `true`                                         |
| `serviceAccount.name`        | Service account name                       | `""`                                           |
| `grpc.address`               | gRPC server bind address                   | `":50051"`                                     |
| `grpc.tls.enabled`           | Enable TLS for gRPC                        | `false`                                        |
| `openstack.auth.authUrl`     | OpenStack auth URL                         | `""`                                           |
| `openstack.auth.username`    | OpenStack username                         | `""`                                           |
| `openstack.auth.password`    | OpenStack password                         | `""`                                           |
| `openstack.auth.projectName` | OpenStack project name                     | `""`                                           |
| `openstack.auth.region`      | OpenStack region                           | `""`                                           |
| `openstack.existingSecret`   | Existing secret with OpenStack credentials | `""`                                           |
| `resources.limits.cpu`       | CPU limit                                  | `500m`                                         |
| `resources.limits.memory`    | Memory limit                               | `512Mi`                                        |
| `resources.requests.cpu`     | CPU request                                | `100m`                                         |
| `resources.requests.memory`  | Memory request                             | `128Mi`                                        |
| `networkPolicy.enabled`      | Enable network policies                    | `false`                                        |
| `rbac.create`                | Create RBAC resources                      | `true`                                         |

## Troubleshooting

### Check Deployment Status

```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=openstack-autoscaler
kubectl logs -n kube-system deployment/openstack-autoscaler
````

### Test gRPC Connectivity

```bash
# Run test
helm test openstack-autoscaler -n kube-system

# Manual test from cluster autoscaler pod
kubectl exec -it deployment/cluster-autoscaler -n kube-system -- /bin/sh
nc -z openstack-autoscaler.kube-system.svc.cluster.local 50051
```

### Debug OpenStack Connectivity

```bash
# Check OpenStack credentials
kubectl get secret openstack-autoscaler-openstack -n kube-system -o yaml

# Test from pod
kubectl exec -it deployment/openstack-autoscaler -n kube-system -- /bin/sh
openstack --help  # if openstack CLI is available in container
```

### Common Issues

1. **gRPC Connection Failed**

   - Check service and endpoint configuration
   - Verify network policies allow traffic
   - Ensure correct port configuration

2. **OpenStack Authentication Failed**

   - Verify credentials in secret
   - Check OpenStack endpoint accessibility
   - Validate domain names and project settings

3. **Node Group Discovery Issues**
   - Check cluster autoscaler node-group-auto-discovery configuration
   - Verify OpenStack flavor and image availability
   - Review autoscaler logs for specific errors

## Examples

### Production Configuration

```yaml
# production-values.yaml
image:
  repository: ghcr.io/bucher-brothers/openstack-autoscaler
  tag: "1.0.0"
  pullPolicy: IfNotPresent

openstack:
  existingSecret: "production-openstack-creds"

grpc:
  tls:
    enabled: true
    cert: "/certs/tls.crt"
    key: "/certs/tls.key"

resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

networkPolicy:
  enabled: true

podDisruptionBudget:
  enabled: true
  minAvailable: 1

securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]

nodeSelector:
  kubernetes.io/os: linux
  node-role.kubernetes.io/worker: ""

tolerations:
  - key: "node-role.kubernetes.io/master"
    operator: "Equal"
    effect: "NoSchedule"

volumes:
  - name: tls-certs
    secret:
      secretName: openstack-autoscaler-tls

volumeMounts:
  - name: tls-certs
    mountPath: /certs
    readOnly: true
```

## Project Architecture

The OpenStack Autoscaler implements the External gRPC Protocol for seamless integration with Kubernetes Cluster Autoscaler:

```
┌─────────────────────────┐    gRPC     ┌──────────────────────────┐
│   Cluster Autoscaler    │◄──────────►│  OpenStack Autoscaler    │
│                         │             │                          │
│  - Scale Up/Down Logic  │             │  - gRPC Server           │
│  - Node Management      │             │  - OpenStack Provider    │
│  - External gRPC Client │             │  - Node Groups           │
└─────────────────────────┘             └──────────────────────────┘
                                                    │
                                                    │ gophercloud v2
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

## Development & Testing

For development and testing outside of Helm:

### Alternative Installation Methods

<details>
<summary>🔧 Local Development</summary>

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

### Development Commands

```bash
# Generate Protobuf code
make proto

# Run tests
make test

# Format code
make fmt

# Linting
make lint

# Helm commands
helm lint ./helm/openstack-autoscaler
helm template ./helm/openstack-autoscaler --values your-values.yaml
helm install --dry-run openstack-autoscaler ./helm/openstack-autoscaler
```

## Complete Project Structure

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

## Advanced Troubleshooting

### Helm Debugging

```bash
# List releases
helm list -n kube-system

# Check release status
helm status openstack-autoscaler -n kube-system

# Upgrade releases
helm upgrade openstack-autoscaler ./helm/openstack-autoscaler -n kube-system

# Uninstall
helm uninstall openstack-autoscaler -n kube-system
helm uninstall cluster-autoscaler -n kube-system

# Debug template rendering
helm template ./helm/openstack-autoscaler --values your-values.yaml
```

### gRPC Connection Debugging

```bash
# Check if OpenStack Autoscaler is running
kubectl get pods -n kube-system -l app.kubernetes.io/name=openstack-autoscaler

# Test gRPC connectivity from cluster autoscaler
kubectl exec -n kube-system deployment/cluster-autoscaler -- \
  nc -z openstack-autoscaler.kube-system.svc.cluster.local 50051

# Check service endpoints
kubectl get endpoints openstack-autoscaler -n kube-system
```

### OpenStack Authentication Debugging

```bash
# Check OpenStack credentials
kubectl get secret -n kube-system openstack-autoscaler-openstack -o yaml

# View autoscaler logs with debug level
kubectl logs -n kube-system deployment/openstack-autoscaler -f

# Test OpenStack connectivity from pod
kubectl exec -it deployment/openstack-autoscaler -n kube-system -- /bin/sh
```

### Cluster Autoscaler Debugging

```bash
# Check cluster autoscaler logs
kubectl logs -n kube-system deployment/cluster-autoscaler -f

# Verify node group discovery
kubectl logs -n kube-system deployment/cluster-autoscaler | grep "node group"

# Check scaling decisions
kubectl logs -n kube-system deployment/cluster-autoscaler | grep -i "scale"
```

## 🤝 Contributing

Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](../../LICENSE) file for details.

## 🆘 Support

- **GitHub Repository**: https://github.com/bucher-brothers/openstack-autoscaler
- **Issues & Bug Reports**: https://github.com/bucher-brothers/openstack-autoscaler/issues
- **Discussions**: https://github.com/bucher-brothers/openstack-autoscaler/discussions

If you encounter any problems or have questions, please create an issue in the GitHub repository.

## 🏢 About Bucher Brothers

Bucher Brothers is a technology consulting company specializing in cloud-native solutions, Kubernetes, and DevOps practices.

**Made with ❤️ by [Bucher Brothers](https://github.com/Bucher-Brothers)**
