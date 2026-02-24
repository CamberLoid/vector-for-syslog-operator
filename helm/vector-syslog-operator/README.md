# Vector Syslog Operator Helm Chart

A Helm chart for deploying the Vector Syslog Operator on Kubernetes.

## Prerequisites

- Kubernetes 1.25+
- Helm 3.8+

## Installation

### Add the repository (if published)

```bash
helm repo add vector-syslog-operator https://camber.github.io/vector-syslog-operator
helm repo update
```

### Install the chart

```bash
# Install with default values
helm install vector-syslog-operator ./helm/vector-syslog-operator \
  --namespace vector-syslog-operator-system \
  --create-namespace

# Install with custom values
helm install vector-syslog-operator ./helm/vector-syslog-operator \
  --namespace vector-syslog-operator-system \
  --create-namespace \
  --set image.tag=v0.1.0 \
  --set resources.limits.memory=512Mi
```

## Configuration

### Common values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `ghcr.io/camber/vector-syslog-operator` |
| `image.tag` | Operator image tag | Chart `appVersion` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `256Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |

### CRD options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `crds.install` | Install CRDs | `true` |
| `crds.keep` | Keep CRDs on uninstall | `false` |

### Manager options

| Parameter | Description | Default |
|-----------|-------------|---------|
| `manager.leaderElection.enabled` | Enable leader election | `true` |
| `manager.metrics.enabled` | Enable metrics endpoint | `true` |
| `manager.metrics.port` | Metrics port | `8443` |

## Usage Example

After installing the operator, create a VectorSyslogConfiguration:

```yaml
apiVersion: vectorsyslog.lab.camber.moe/v1alpha1
kind: VectorSocketSource
metadata:
  name: tcp-logs
  namespace: default
spec:
  mode: tcp
  port: 5140
  labels:
    source: myapp
---
apiVersion: vectorsyslog.lab.camber.moe/v1alpha1
kind: VectorSyslogConfiguration
metadata:
  name: default
  namespace: default
spec:
  globalPipeline:
    sinks:
      console:
        type: console
        inputs: $$VectorSyslogOperatorSources$$
        encoding:
          codec: json
  service:
    type: LoadBalancer
```

## Upgrading

```bash
helm upgrade vector-syslog-operator ./helm/vector-syslog-operator \
  --namespace vector-syslog-operator-system
```

## Uninstalling

```bash
helm uninstall vector-syslog-operator \
  --namespace vector-syslog-operator-system
```

## Development

### Lint the chart

```bash
helm lint ./helm/vector-syslog-operator
```

### Template rendering

```bash
helm template vector-syslog-operator ./helm/vector-syslog-operator
```

### Package the chart

```bash
helm package ./helm/vector-syslog-operator
```
