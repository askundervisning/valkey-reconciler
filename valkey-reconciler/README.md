# Valkey Reconciler

A Kubernetes controller that listens for Redis Sentinel events and automatically updates pod labels to track the current Redis master instance.

## Overview

The Valkey Reconciler monitors Redis Sentinel for `+switch-master` events and maintains accurate labeling of Kubernetes pods to identify which pod is currently serving as the Redis master. This enables services to dynamically route traffic to the correct master instance during failover scenarios.

## How It Works

1. **Initial Master Detection**: On startup, queries Redis Sentinel to identify the current master
2. **Pod Labeling**: Updates Kubernetes pod labels to mark the master pod with configurable labels
3. **Event Monitoring**: Subscribes to Redis Sentinel pub/sub for `+switch-master` events
4. **Automatic Failover**: When a master switch occurs, removes the master label from the old pod and applies it to the new master pod

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Redis Sentinel │───►│ Valkey Reconciler│───►│ Kubernetes Pods │
│    (Events)     │    │  (Controller)    │    │   (Labels)      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Configuration

The reconciler is configured via environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `VALKEY_SENTINEL_HOST` | Redis Sentinel hostname | - | ✅ |
| `VALKEY_SENTINEL_PORT` | Redis Sentinel port | `26379` | ❌ |
| `VALKEY_SENTINEL_PASSWORD` | Redis Sentinel password | - | ✅ |
| `VALKEY_MASTER_NAME` | Redis master service name | `myprimary` | ❌ |
| `POD_NAMESPACE` | Kubernetes namespace | `default` | ❌ |
| `MASTER_POD_LABEL_NAME` | Label key for master pods | `valkey-master` | ❌ |
| `MASTER_POD_LABEL_VALUE` | Label value for master pods | `true` | ❌ |

## Deployment

### Prerequisites

- Kubernetes cluster with RBAC enabled
- Redis cluster with Sentinel running
- Secret containing Redis Sentinel password

### Deploy using Kubernetes manifests

1. **Create the service account and RBAC permissions**:
   ```bash
   kubectl apply -f service-account.yaml
   ```

2. **Create the secret with Redis password**:
   ```bash
   kubectl create secret generic valkey-cluster --from-literal=password=your-redis-password
   ```

3. **Deploy the reconciler**:
   ```bash
   kubectl apply -f redis-follower-deployment.yaml
   ```

4. **Deploy the service** (routes traffic to master pod):
   ```bash
   kubectl apply -f valkey-main.yaml
   ```

### Build from source

```bash
# Build the binary
go build

# Build Docker image
docker build -t valkey-reconciler:latest .
```

## Service Discovery

The reconciler works in conjunction with a Kubernetes Service that uses label selectors to route traffic to the current master:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: valkey
spec:
  selector:
    vk-master: "true"  # Matches the MASTER_POD_LABEL_NAME/VALUE
  ports:
  - port: 6379
    targetPort: 6379
```

## RBAC Permissions

The reconciler requires the following Kubernetes permissions:

- `list` - to discover pods with `app.kubernetes.io/name=valkey` label
- `update` - to modify pod labels
- `patch` - to apply label changes

## Monitoring

The reconciler logs all major events:

- Connection establishment with Sentinel
- Master discovery and changes
- Pod label updates
- Connection failures and retries

## Resilience Features

- **Automatic Reconnection**: Reconnects to Sentinel on connection loss
- **Health Checks**: Validates Sentinel connectivity with ping
- **TLS Support**: Connects to Sentinel with TLS (with `InsecureSkipVerify`)
- **Graceful Error Handling**: Continues operation despite individual pod update failures

## Development

### Testing

```bash
# Run all tests
go test

# Run specific test
go test -run '^TestName$'
```

### Code Style

Follow Go conventions and project guidelines in `CLAUDE.md`.

## Troubleshooting

### Common Issues

1. **Connection failures to Sentinel**
   - Verify `VALKEY_SENTINEL_HOST` and `VALKEY_SENTINEL_PORT`
   - Check network connectivity and firewall rules
   - Validate `VALKEY_SENTINEL_PASSWORD`

2. **Pod label updates failing**
   - Verify RBAC permissions are correctly applied
   - Check that pods have the `app.kubernetes.io/name=valkey` label
   - Ensure the reconciler is running in the correct namespace

3. **Service not routing to master**
   - Verify the Service selector matches `MASTER_POD_LABEL_NAME`/`VALUE`
   - Check that only one pod has the master label

### Logs

Monitor reconciler logs for detailed operation information:

```bash
kubectl logs -f deployment/valkey-reconciler
```