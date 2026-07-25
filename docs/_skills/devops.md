---
layout: skill
title: DevOps
name: devops
category: ops
description: CI/CD pipelines, containerization, infrastructure as code, and deployment automation best practices.
---

# DevOps

Guidelines for building reliable CI/CD pipelines, containerization, and deployment automation.

## When to Activate

- Setting up CI/CD pipelines
- Writing Dockerfiles
- Creating Kubernetes manifests
- Infrastructure as Code (Terraform, Pulumi)

## Docker Best Practices

### Multi-stage builds

```dockerfile
# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
RUN npm run build

# Production stage
FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
USER node
EXPOSE 3000
CMD ["node", "dist/server.js"]
```

### Security hardening

```dockerfile
# GOOD - specific version, non-root user
FROM node:20.10-alpine

# Create non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

WORKDIR /app
COPY --chown=appuser:appgroup . .
USER appuser

# BAD - root user, latest tag
FROM node:latest
COPY . .
# Running as root!
```

### .dockerignore

```dockerignore
node_modules
.git
.env*
*.log
Dockerfile*
docker-compose*
.dockerignore
README.md
tests/
coverage/
```

## GitHub Actions

### Standard CI workflow

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-node@v4
        with:
          node-version: 20
          cache: 'npm'
      
      - run: npm ci
      - run: npm run lint
      - run: npm run test
      - run: npm run build
```

### CD with environment protection

{% raw %}
```yaml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production
    steps:
      - uses: actions/checkout@v4
      
      - name: Deploy to production
        env:
          DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}
        run: |
          ./deploy.sh
```
{% endraw %}

### Caching for speed

{% raw %}
```yaml
- name: Cache dependencies
  uses: actions/cache@v4
  with:
    path: ~/.npm
    key: npm-${{ hashFiles('package-lock.json') }}
    restore-keys: npm-

- name: Cache build
  uses: actions/cache@v4
  with:
    path: .next/cache
    key: nextjs-${{ hashFiles('package-lock.json') }}-${{ hashFiles('**/*.ts') }}
```
{% endraw %}

## Kubernetes

### Deployment manifest

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels:
    app: api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api
  template:
    metadata:
      labels:
        app: api
    spec:
      containers:
        - name: api
          image: myapp:v1.2.3  # Pin version, never :latest
          ports:
            - containerPort: 3000
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 3000
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 5
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: url
```

### Service and Ingress

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  selector:
    app: api
  ports:
    - port: 80
      targetPort: 3000
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - api.example.com
      secretName: api-tls
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: api
                port:
                  number: 80
```

## Terraform

### Standard structure

```
terraform/
├── main.tf
├── variables.tf
├── outputs.tf
├── providers.tf
├── versions.tf
└── modules/
    └── vpc/
        ├── main.tf
        ├── variables.tf
        └── outputs.tf
```

### Remote state

```hcl
terraform {
  backend "s3" {
    bucket         = "terraform-state-prod"
    key            = "app/terraform.tfstate"
    region         = "us-east-1"
    encrypt        = true
    dynamodb_table = "terraform-locks"
  }
}
```

### Modular resources

```hcl
module "vpc" {
  source = "./modules/vpc"
  
  name            = "production"
  cidr            = "10.0.0.0/16"
  azs             = ["us-east-1a", "us-east-1b"]
  private_subnets = ["10.0.1.0/24", "10.0.2.0/24"]
  public_subnets  = ["10.0.101.0/24", "10.0.102.0/24"]
  
  tags = {
    Environment = "production"
    Terraform   = "true"
  }
}
```

## Secrets in CI/CD

### Never commit secrets

{% raw %}
```yaml
# GOOD - use secrets
env:
  DATABASE_URL: ${{ secrets.DATABASE_URL }}

# BAD - hardcoded
env:
  DATABASE_URL: postgres://user:password@host/db
```
{% endraw %}

### Rotate secrets regularly

```yaml
# Secret rotation workflow
name: Rotate Secrets

on:
  schedule:
    - cron: '0 0 1 * *'  # Monthly

jobs:
  rotate:
    runs-on: ubuntu-latest
    steps:
      - name: Rotate API keys
        run: ./scripts/rotate-secrets.sh
```

## Monitoring & Alerts

### Health check endpoints

```typescript
// /health/live - is the process running?
app.get('/health/live', (req, res) => {
  res.json({ status: 'alive' });
});

// /health/ready - can it serve traffic?
app.get('/health/ready', async (req, res) => {
  const checks = {
    database: await checkDatabase(),
    redis: await checkRedis(),
  };
  
  const healthy = Object.values(checks).every(Boolean);
  res.status(healthy ? 200 : 503).json({ checks });
});
```

### Structured logging for observability

```typescript
logger.info({
  event: 'request_completed',
  method: req.method,
  path: req.path,
  status: res.statusCode,
  duration_ms: duration,
  trace_id: req.headers['x-trace-id'],
});
```
