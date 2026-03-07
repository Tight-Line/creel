# Deployment Guide

## Helm chart installation

The primary distribution is a Helm chart at `deploy/helm/creel/`.

```bash
helm install creel deploy/helm/creel/ \
  --set metadata.postgresUrl="postgres://user:pass@host:5432/creel?sslmode=require" \
  --set auth.apiKeys[0].name=bootstrap \
  --set auth.apiKeys[0].keyHash="<hash>" \
  --set auth.apiKeys[0].principal=admin
```

### Embedded PostgreSQL (CloudNativePG)

By default, the chart includes a PostgreSQL dependency via CloudNativePG. To use it:

```yaml
postgresql:
  enabled: true
```

### External PostgreSQL

To use an existing PostgreSQL instance (must have pgvector installed):

```yaml
postgresql:
  enabled: false

metadata:
  postgresUrl: "postgres://user:pass@your-host:5432/creel?sslmode=require"
```

## Configuration reference

Creel is configured via a YAML file. All settings can also be overridden with environment variables using the `CREEL_` prefix (e.g., `CREEL_METADATA_POSTGRES_URL`).

### server

| Key | Default | Description |
|-----|---------|-------------|
| `grpc_port` | 8443 | gRPC listen port |
| `rest_port` | 8080 | REST (grpc-gateway) listen port |
| `metrics_port` | 9090 | Prometheus metrics port |

### auth

| Key | Default | Description |
|-----|---------|-------------|
| `principal_claim` | `sub` | JWT claim used as principal identity |
| `groups_claim` | | JWT claim containing group memberships |

#### auth.providers (OIDC)

```yaml
auth:
  providers:
    - issuer: "https://accounts.google.com"
      audience: "your-client-id.apps.googleusercontent.com"
```

Each provider specifies an `issuer` (the OIDC discovery URL) and `audience` (your client ID).

#### auth.api_keys

```yaml
auth:
  api_keys:
    - name: my-service
      key_hash: "sha256:<hash>"
      principal: "service-account"
```

API keys are stored as SHA-256 hashes. Generate keys with `creel bootstrap-key`.

### metadata

| Key | Default | Description |
|-----|---------|-------------|
| `postgres_url` | (required) | PostgreSQL connection string |

### vector_backend

| Key | Default | Description |
|-----|---------|-------------|
| `type` | `pgvector` | Vector backend type |
| `config` | | Backend-specific configuration (map) |

When `type` is `pgvector`, the backend uses the same PostgreSQL connection as metadata.

### embedding

| Key | Default | Description |
|-----|---------|-------------|
| `provider` | | Embedding provider (for server-side embedding; future) |
| `model` | | Model name |
| `api_key` | | Provider API key |

### links

| Key | Default | Description |
|-----|---------|-------------|
| `auto_link_on_ingest` | | Enable automatic linking on chunk ingestion |
| `auto_link_threshold` | | Similarity threshold for auto-linking |
| `max_traversal_depth` | | Maximum link traversal depth for search |

### compaction

| Key | Default | Description |
|-----|---------|-------------|
| `retain_compacted_chunks` | | Keep original chunks after compaction |

## OIDC provider setup

### Google

1. Create an OAuth 2.0 client ID in the Google Cloud Console.
2. Configure:

```yaml
auth:
  principal_claim: sub
  groups_claim: hd
  providers:
    - issuer: "https://accounts.google.com"
      audience: "your-client-id.apps.googleusercontent.com"
```

### Generic OIDC

Any provider with a standard `/.well-known/openid-configuration` endpoint works:

```yaml
auth:
  providers:
    - issuer: "https://your-idp.example.com"
      audience: "creel"
```

## Docker Compose for staging

The included `docker-compose.yml` runs PostgreSQL/pgvector and the Creel server:

```bash
docker compose up -d
```

Services:

- **postgres**: pgvector/pgvector:pg17 on port 5432
- **creel**: server on ports 8443 (gRPC), 8080 (REST), 9090 (metrics)

The server runs with `--migrate` to apply schema migrations on startup.

## Security defaults

### Docker image

The Docker image runs as a non-root `creel` user. If you mount external volumes (config files, secrets), ensure the files are readable by this unprivileged user.

### Helm chart

The chart sets the following security defaults:

- `automountServiceAccountToken: false` on the pod spec (no Kubernetes API access from the pod)
- Default resource limits: 100m/128Mi requests, 500m/512Mi limits. Override in `values.yaml` under `resources`.

## Health checks

The Docker image includes a health check that runs `creel-cli health` every 30 seconds. The gRPC health endpoint is available at the standard gRPC health checking protocol.

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8443 | gRPC | Primary API |
| 8080 | HTTP | REST API (grpc-gateway) |
| 9090 | HTTP | Prometheus metrics |
