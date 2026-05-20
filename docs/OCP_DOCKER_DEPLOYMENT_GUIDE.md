# LLM Gateway OpenShift Deployment Guide (Docker Image)

This guide explains how to deploy LLM Gateway on OpenShift Container Platform (OCP) by building Docker images and running them as OpenShift workloads.

## 1. Prerequisites

- OpenShift cluster access
- `oc` CLI installed and logged in
- Image registry access (OpenShift internal registry or external)
- Project/namespace created (example: `llm-gateway`)
- Storage class available for MariaDB persistence

## 2. Deployment Architecture

Recommended components in OCP:

- Frontend Deployment + Service + Route
- Backend Deployment + Service + Route (or internal service only)
- MariaDB StatefulSet/Deployment + PVC + Service
- Redis Deployment + Service
- Config via ConfigMap and Secret

## 3. Login and Select Project

```bash
oc login https://api.<cluster-domain>:6443
oc new-project llm-gateway
# or
oc project llm-gateway
```

## 4. Build Docker Images

Build from your local source repo:

```bash
cd /path/to/Artemis-llmgateway

docker build -t llm-gateway-backend:latest ./backend
docker build -t llm-gateway-frontend:latest ./frontend
```

## 5. Push Images to Registry

Use one of the following options.

### Option A: Push to OpenShift internal registry

```bash
oc registry login

# Tag images for internal registry
BACKEND_IMAGE=image-registry.openshift-image-registry.svc:5000/llm-gateway/llm-gateway-backend:latest
FRONTEND_IMAGE=image-registry.openshift-image-registry.svc:5000/llm-gateway/llm-gateway-frontend:latest

docker tag llm-gateway-backend:latest ${BACKEND_IMAGE}
docker tag llm-gateway-frontend:latest ${FRONTEND_IMAGE}

docker push ${BACKEND_IMAGE}
docker push ${FRONTEND_IMAGE}
```

### Option B: Push to external registry

Tag and push using your registry naming convention, then use those image URLs in deployment YAML.

## 6. Create Configuration and Secrets

Create backend secret values:

```bash
oc create secret generic llm-gateway-secrets \
  --from-literal=DB_PASSWORD='gatwaysecret' \
  --from-literal=DB_ROOT_PASSWORD='rootsecret' \
  --from-literal=JWT_SECRET='replace_with_long_random_secret' \
  --from-literal=DEFAULT_ADMIN_PASSWORD_BCRYPT='$2a$10$replace_with_bcrypt_hash'
```

Create config values:

```bash
oc create configmap llm-gateway-config \
  --from-literal=DB_HOST='mariadb' \
  --from-literal=DB_PORT='3306' \
  --from-literal=DB_NAME='llm_gatway' \
  --from-literal=DB_USER='gatway' \
  --from-literal=PORT='8080' \
  --from-literal=DEFAULT_ADMIN_ENABLED='true' \
  --from-literal=DEFAULT_ADMIN_ID='00000000-0000-0000-0000-000000000001' \
  --from-literal=DEFAULT_ADMIN_EMAIL='admin@llm-gatway.local' \
  --from-literal=FRONTEND_ORIGIN='https://llm-gateway-frontend.apps.<cluster-domain>' \
  --from-literal=NEXT_PUBLIC_API_URL='https://llm-gateway-backend.apps.<cluster-domain>'
```

## 7. Deploy Data Services

### 7.1 MariaDB with persistent volume

```bash
oc apply -f - <<'EOF'
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mariadb-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mariadb
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mariadb
  template:
    metadata:
      labels:
        app: mariadb
    spec:
      containers:
      - name: mariadb
        image: mariadb:11.3
        ports:
        - containerPort: 3306
        env:
        - name: MARIADB_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: llm-gateway-secrets
              key: DB_ROOT_PASSWORD
        - name: MARIADB_DATABASE
          valueFrom:
            configMapKeyRef:
              name: llm-gateway-config
              key: DB_NAME
        - name: MARIADB_USER
          valueFrom:
            configMapKeyRef:
              name: llm-gateway-config
              key: DB_USER
        - name: MARIADB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: llm-gateway-secrets
              key: DB_PASSWORD
        volumeMounts:
        - name: data
          mountPath: /var/lib/mysql
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mariadb-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: mariadb
spec:
  selector:
    app: mariadb
  ports:
  - port: 3306
    targetPort: 3306
EOF
```

### 7.2 Redis

```bash
oc apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7.2-alpine
        ports:
        - containerPort: 6379
        args: ["redis-server", "--appendonly", "yes"]
---
apiVersion: v1
kind: Service
metadata:
  name: redis
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
EOF
```

## 8. Deploy Backend and Frontend

Apply backend + frontend manifests. Replace image references with your registry paths.

```bash
oc apply -f - <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llm-gateway-backend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: llm-gateway-backend
  template:
    metadata:
      labels:
        app: llm-gateway-backend
    spec:
      containers:
      - name: backend
        image: image-registry.openshift-image-registry.svc:5000/llm-gateway/llm-gateway-backend:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_HOST
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DB_HOST}}
        - name: DB_PORT
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DB_PORT}}
        - name: DB_NAME
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DB_NAME}}
        - name: DB_USER
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DB_USER}}
        - name: DB_PASSWORD
          valueFrom: {secretKeyRef: {name: llm-gateway-secrets, key: DB_PASSWORD}}
        - name: JWT_SECRET
          valueFrom: {secretKeyRef: {name: llm-gateway-secrets, key: JWT_SECRET}}
        - name: DEFAULT_ADMIN_ENABLED
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DEFAULT_ADMIN_ENABLED}}
        - name: DEFAULT_ADMIN_ID
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DEFAULT_ADMIN_ID}}
        - name: DEFAULT_ADMIN_EMAIL
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: DEFAULT_ADMIN_EMAIL}}
        - name: DEFAULT_ADMIN_PASSWORD_BCRYPT
          valueFrom: {secretKeyRef: {name: llm-gateway-secrets, key: DEFAULT_ADMIN_PASSWORD_BCRYPT}}
        - name: FRONTEND_ORIGIN
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: FRONTEND_ORIGIN}}
        - name: PORT
          valueFrom: {configMapKeyRef: {name: llm-gateway-config, key: PORT}}
---
apiVersion: v1
kind: Service
metadata:
  name: llm-gateway-backend
spec:
  selector:
    app: llm-gateway-backend
  ports:
  - port: 8080
    targetPort: 8080
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: llm-gateway-backend
spec:
  to:
    kind: Service
    name: llm-gateway-backend
  port:
    targetPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llm-gateway-frontend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: llm-gateway-frontend
  template:
    metadata:
      labels:
        app: llm-gateway-frontend
    spec:
      containers:
      - name: frontend
        image: image-registry.openshift-image-registry.svc:5000/llm-gateway/llm-gateway-frontend:latest
        ports:
        - containerPort: 3000
        env:
        - name: NEXT_PUBLIC_API_URL
          valueFrom:
            configMapKeyRef:
              name: llm-gateway-config
              key: NEXT_PUBLIC_API_URL
---
apiVersion: v1
kind: Service
metadata:
  name: llm-gateway-frontend
spec:
  selector:
    app: llm-gateway-frontend
  ports:
  - port: 3000
    targetPort: 3000
---
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: llm-gateway-frontend
spec:
  to:
    kind: Service
    name: llm-gateway-frontend
  port:
    targetPort: 3000
EOF
```

## 9. Verify Deployment

```bash
oc get pods
oc get svc
oc get route
```

Check backend health:

```bash
curl https://llm-gateway-backend.apps.<cluster-domain>/health
```

Open frontend route in browser and log in.

## 10. Rolling Updates

Build and push new images, then restart deployment:

```bash
# after pushing updated :latest images
oc rollout restart deployment/llm-gateway-backend
oc rollout restart deployment/llm-gateway-frontend

oc rollout status deployment/llm-gateway-backend
oc rollout status deployment/llm-gateway-frontend
```

## 11. Security and Operations Recommendations

- Move all secrets to OpenShift Secret objects only
- Use image tags per release (avoid only `latest`)
- Configure liveness/readiness probes
- Set resource requests/limits
- Enable route TLS and enforce HTTPS
- Integrate centralized logs and metrics

## 12. Troubleshooting

View logs:

```bash
oc logs deployment/llm-gateway-backend
oc logs deployment/llm-gateway-frontend
oc logs deployment/mariadb
```

Describe failed pod:

```bash
oc describe pod <pod-name>
```

Common issues:

- Image pull permission denied
- Wrong DB credentials in secret
- Incorrect `NEXT_PUBLIC_API_URL` or `FRONTEND_ORIGIN`
- Route hostnames not matching cluster domain

## 13. File References

- Backend Docker image build: [backend/Dockerfile](backend/Dockerfile)
- Frontend Docker image build: [frontend/Dockerfile](frontend/Dockerfile)
- Local compose baseline: [docker-compose.yml](docker-compose.yml)
- Environment keys: [.env.example](.env.example)
