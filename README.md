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
- ✅ **OpenStack Integration**: Native integration via gophercloud
- ✅ **Multi-NodeGroup Support**: Support for multiple node groups with different configurations
- ✅ **Cloud-Init Support**: Flexible server initialization via Cloud-Init/User Data
- ✅ **TLS Support**: Secure gRPC communication with TLS
- ✅ **Multi-Architecture**: Support for AMD64 and ARM64
- ✅ **Container Ready**: Docker images available

## Quick Start

### 1. Configuration

Create a configuration file based on the example:

```bash
cp config.yaml.example config.yaml
# Edit config.yaml with your OpenStack credentials and node group configurations
```

### 2. Build & Run

```bash
# Install dependencies
make deps

# Build binary
make build

# Start with configuration file
make run

# Or with environment variables
export OS_AUTH_URL="https://keystone.example.com:5000/v3"
export OS_USERNAME="your-username"
export OS_PASSWORD="your-password"
export OS_PROJECT_NAME="your-project"
export OS_REGION_NAME="RegionOne"
make run-env
```

### 3. Docker

```bash
# Build Docker image
make docker-build-amd64

# Run with Docker
docker run -v $(pwd)/config.yaml:/config.yaml \
  ghcr.io/bucher-brothers/openstack-autoscaler:latest \
  --config=/config.yaml
```

## Configuration

### OpenStack Cloud Configuration

```yaml
cloud:
  auth_url: "https://keystone.example.com:5000/v3"
  username: "your-username"
  password: "your-password"
  project_name: "your-project"
  user_domain_name: "Default"
  project_domain_name: "Default"
  region: "RegionOne"
  interface: "public"
```

### Node Groups Configuration

**Important**: Node groups are **not** configured in this service's configuration file. Instead, they are dynamically managed by the Kubernetes Cluster Autoscaler through the external-grpc protocol.

The Cluster Autoscaler will:

1. Send node group definitions via gRPC calls
2. Request scaling operations (increase/decrease nodes)
3. Provide all necessary OpenStack configuration (flavor, image, network, etc.)

This service only needs the **OpenStack cloud credentials** in its configuration.

### Environment Variables

Alternatively to the configuration file, OpenStack credentials can be set via environment variables:

- `OS_AUTH_URL`: OpenStack Authentication URL
- `OS_USERNAME`: OpenStack Username
- `OS_PASSWORD`: OpenStack Password
- `OS_PROJECT_NAME`: OpenStack Project Name
- `OS_USER_DOMAIN_NAME`: User Domain (Default: "Default")
- `OS_PROJECT_DOMAIN_NAME`: Project Domain (Default: "Default")
- `OS_REGION_NAME`: OpenStack Region
- `OS_INTERFACE`: Interface Type (public/internal/admin)

## Kubernetes Cluster Autoscaler Integration

### 1. Service Account und RBAC

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: cluster-autoscaler
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: cluster-autoscaler
rules:
  - apiGroups: [""]
    resources: ["events", "endpoints"]
    verbs: ["create", "patch"]
  - apiGroups: [""]
    resources: ["pods/eviction"]
    verbs: ["create"]
  - apiGroups: [""]
    resources: ["pods/status"]
    verbs: ["update"]
  - apiGroups: [""]
    resources: ["endpoints"]
    resourceNames: ["cluster-autoscaler"]
    verbs: ["get", "update"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["watch", "list", "get", "update"]
  - apiGroups: [""]
    resources:
      [
        "pods",
        "services",
        "replicationcontrollers",
        "persistentvolumeclaims",
        "persistentvolumes",
      ]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["extensions"]
    resources: ["replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["watch", "list"]
  - apiGroups: ["apps"]
    resources: ["statefulsets", "replicasets", "daemonsets"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses", "csinodes"]
    verbs: ["watch", "list", "get"]
  - apiGroups: ["batch", "extensions"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "patch"]
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["create"]
  - apiGroups: ["coordination.k8s.io"]
    resourceNames: ["cluster-autoscaler"]
    resources: ["leases"]
    verbs: ["get", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: cluster-autoscaler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-autoscaler
subjects:
  - kind: ServiceAccount
    name: cluster-autoscaler
    namespace: kube-system
```

### 2. OpenStack Autoscaler Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openstack-autoscaler
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: openstack-autoscaler
  template:
    metadata:
      labels:
        app: openstack-autoscaler
    spec:
      containers:
        - name: openstack-autoscaler
          image: ghcr.io/bucher-brothers/openstack-autoscaler:latest
          ports:
            - containerPort: 8086
          env:
            - name: OS_AUTH_URL
              value: "https://keystone.example.com:5000/v3"
            - name: OS_USERNAME
              valueFrom:
                secretKeyRef:
                  name: openstack-credentials
                  key: username
            - name: OS_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: openstack-credentials
                  key: password
            - name: OS_PROJECT_NAME
              value: "your-project"
            - name: OS_REGION_NAME
              value: "RegionOne"
          volumeMounts:
            - name: config
              mountPath: /config.yaml
              subPath: config.yaml
          command:
            - ./openstack-autoscaler
          args:
            - --config=/config.yaml
            - --address=:8086
      volumes:
        - name: config
          configMap:
            name: openstack-autoscaler-config
---
apiVersion: v1
kind: Service
metadata:
  name: openstack-autoscaler
  namespace: kube-system
spec:
  ports:
    - port: 8086
      targetPort: 8086
      name: grpc
  selector:
    app: openstack-autoscaler
```

### 3. Cluster Autoscaler Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    metadata:
      labels:
        app: cluster-autoscaler
    spec:
      serviceAccountName: cluster-autoscaler
      containers:
        - image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.27.0
          name: cluster-autoscaler
          resources:
            limits:
              cpu: 100m
              memory: 300Mi
            requests:
              cpu: 100m
              memory: 300Mi
          command:
            - ./cluster-autoscaler
            - --v=4
            - --stderrthreshold=info
            - --cloud-provider=externalgrpc
            - --cloud-config=/etc/kubernetes/cloud-config
            - --nodes=1:10:worker-nodes
            - --nodes=0:5:gpu-nodes
            - --grpc-client-cert-file=/etc/ssl/grpc/client.crt
            - --grpc-client-key-file=/etc/ssl/grpc/client.key
            - --grpc-ca-cert-file=/etc/ssl/grpc/ca.crt
            - --grpc-provider-address=openstack-autoscaler.kube-system.svc.cluster.local:8086
          volumeMounts:
            - name: ssl-certs
              mountPath: /etc/ssl/grpc
              readOnly: true
            - name: cloud-config
              mountPath: /etc/kubernetes/cloud-config
              readOnly: true
      volumes:
        - name: ssl-certs
          secret:
            secretName: grpc-client-certs
        - name: cloud-config
          configMap:
            name: cluster-autoscaler-status
```

## Development

### Generate Protobuf Code

```bash
make proto
```

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Linting

```bash
make lint
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
├── config.yaml.example           # Example configuration
├── go.mod                        # Go module definition
└── README.md                     # Project documentation
```

## Architektur

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
