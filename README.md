# Agnos DevOps Assignment

Production-ready DevOps setup — reliability, security, CI/CD, and observability.

## Architecture Overview

```
┌─────────────────────────────────────────────────────┐
│                   Kubernetes (agnos ns)              │
│                                                     │
│  ┌──────────────────────┐  ┌────────────────────┐   │
│  │   agnos-api (x2)     │  │  agnos-worker (x1) │   │
│  │  GET /health → 200   │  │  runJob() every N  │   │
│  │  HPA: 2–10 replicas  │  │  seconds           │   │
│  └──────────┬───────────┘  └────────────────────┘   │
│             │ ClusterIP :80                          │
└─────────────┼───────────────────────────────────────┘
              │
        (Ingress / LB)
```

**Components**

| Service | Language | Role |
|---------|----------|------|
| `api` | Go + Gin | REST API with `GET /health` |
| `worker` | Go | Background job — updates today's record timestamps |

All logs are **structured JSON** (Go `log/slog`), ready for log aggregators (Loki, CloudWatch, etc.).

---

## Repository Layout

```
agnos-devops/
├── api/                   # API service
│   ├── main.go
│   ├── Dockerfile
│   ├── go.mod / go.sum
├── worker/                # Background worker
│   ├── main.go
│   ├── Dockerfile
│   ├── go.mod
├── envs/                  # Non-secret env config per environment
│   ├── .env.dev
│   ├── .env.uat
│   └── .env.prod
├── k8s/                   # Kubernetes manifests
│   ├── namespace.yaml
│   ├── configmap.yaml
│   ├── api-deployment.yaml
│   ├── api-service.yaml
│   ├── api-hpa.yaml
│   ├── api-pdb.yaml
│   └── worker-deployment.yaml
├── .github/workflows/
│   └── ci.yml             # GitHub Actions pipeline
└── docker-compose.yml     # Local dev with dev env
```

---

## Setup Instructions

### Prerequisites

- Docker & Docker Compose
- Go 1.23+
- kubectl + a Kubernetes cluster (local: kind / minikube)

### Local Development

```bash
# Run both services locally (uses envs/.env.dev)
docker compose up --build

# Health check
curl http://localhost:8080/health
```

### Kubernetes Deploy

```bash
# 1. Apply all manifests (order matters)
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/api-deployment.yaml
kubectl apply -f k8s/api-service.yaml
kubectl apply -f k8s/api-hpa.yaml
kubectl apply -f k8s/api-pdb.yaml
kubectl apply -f k8s/worker-deployment.yaml

# 2. Check rollout
kubectl rollout status deployment/agnos-api -n agnos
kubectl rollout status deployment/agnos-worker -n agnos

# 3. Port-forward for local test
kubectl port-forward svc/agnos-api 8080:80 -n agnos
curl http://localhost:8080/health
```

### Switching Environments

Change `configMapRef.name` in the deployment YAML:

| Environment | ConfigMap name |
|-------------|----------------|
| dev | `agnos-config-dev` |
| uat | `agnos-config-uat` |
| prod | `agnos-config-prod` |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ENV` | `dev` | Environment name (`dev` / `uat` / `prod`) |
| `PORT` | `8080` | API listen port |
| `WORKER_INTERVAL_SECONDS` | `30` | How often the worker runs its job |

---

## CI/CD Pipeline

`.github/workflows/ci.yml` — triggered on push / PR to `main`:

```
Lint (golangci-lint)
  └─► Test (go test -race + coverage)
        └─► Build & Push (Docker → GHCR)  ← main branch only
              └─► Security Scan (Trivy → GitHub Security)
                    └─► Deploy (mocked kubectl rollout)
```

Images are tagged `sha-<short-sha>` and `latest`.

---

## Observability

All services emit **structured JSON logs** via `log/slog`:

```json
{"time":"2025-05-15T10:00:00Z","level":"INFO","msg":"request","method":"GET","path":"/health","status":200,"latency_ms":1}
{"time":"2025-05-15T10:00:30Z","level":"INFO","msg":"job completed","date":"2025-05-15","records_updated":3,"duration_ms":201}
```

**Recommended stack:** Prometheus + Grafana + Loki

Key alerts to configure:
- API error rate > 1% for 5 min
- Worker no log activity for > 2× interval
- Pod restart count > 3 in 10 min (crash loop)

---

## Failure Scenarios

### a. API crashes during peak hours

The HPA keeps **≥2 replicas** running at all times. The PodDisruptionBudget guarantees `minAvailable: 1` during voluntary disruptions (node drain, rolling update). `RollingUpdate` with `maxUnavailable: 0` ensures zero downtime deploys. Kubernetes automatically restarts any crashed pod.

**Action:** Monitor restart count via `kubectl get pods -n agnos`. If persistent, check logs (`kubectl logs`) and roll back with `kubectl rollout undo deployment/agnos-api -n agnos`.

### b. Worker fails and infinitely retries

The worker uses a ticker-based loop (not a crash-retry loop). If `runJob()` panics, the process exits and Kubernetes restarts it (default `restartPolicy: Always`). To prevent runaway restarts, the liveness probe will kill a stuck worker pod after 3 consecutive failures.

**Action:** Check `kubectl describe pod -n agnos` for `CrashLoopBackOff`. Fix the root cause in code and redeploy. Consider adding a dead-letter mechanism or circuit breaker if the job calls external systems.

### c. Bad deployment is released

Use `kubectl rollout undo deployment/agnos-api -n agnos` to immediately roll back to the previous ReplicaSet. The readiness probe prevents the bad version from receiving traffic until it passes health checks — so even a bad deploy won't serve errors if the pod never becomes ready.

**Action:** Set up deployment gating in CI (smoke test against staging before promoting to prod).

### d. Kubernetes node goes down

With **pod anti-affinity** configured, API replicas are spread across nodes. When a node goes down, the scheduler reschedules its pods onto healthy nodes. The PodDisruptionBudget ensures at least one replica stays available during planned maintenance.

**Action:** Ensure the cluster has **≥2 worker nodes** for true HA. Use cluster autoscaler to replace failed nodes automatically in cloud environments.
