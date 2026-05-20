# LLM Gateway Server Implementation Guide

This guide provides step-by-step instructions to deploy LLM Gateway on a Linux server using Docker Compose.

## 1. Prerequisites

- Linux server (Ubuntu 22.04/24.04 recommended)
- 4 vCPU, 8 GB RAM minimum
- 30 GB free disk
- Open ports: 3000 (frontend), 8080 (backend), 3307 (optional DB host access)
- sudo access
- Domain name (optional, recommended for production)

## 2. Install Docker and Docker Compose

Run on the server:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl gnupg lsb-release

sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo $VERSION_CODENAME) stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

sudo usermod -aG docker $USER
newgrp docker

docker --version
docker compose version
```

## 3. Get the Application Code

```bash
mkdir -p /opt/llm_gateway
cd /opt

git clone https://github.com/realtimedetect/Artemis-llmgateway.git llm_gateway
cd /opt/llm_gateway
```

## 4. Create Environment Configuration

Copy and edit environment values:

```bash
cp .env.example .env
nano .env
```

At minimum, update these in `.env`:

- `JWT_SECRET` (long random value)
- `DB_PASSWORD`
- `DB_ROOT_PASSWORD`
- `DEFAULT_ADMIN_PASSWORD_BCRYPT`
- `FRONTEND_ORIGIN` (for your domain)
- `NEXT_PUBLIC_API_URL` (backend URL)

Generate a strong JWT secret:

```bash
openssl rand -hex 32
```

Generate bcrypt hash for admin password:

```bash
docker run --rm httpd:2.4-alpine htpasswd -nbBC 10 "" "YourStrongPassword" | tr -d ':\n'
```

Paste hash into `DEFAULT_ADMIN_PASSWORD_BCRYPT`.

## 5. Start the Platform

```bash
cd /opt/llm_gateway
docker compose pull
docker compose build --no-cache
docker compose up -d
```

Check status:

```bash
docker compose ps
docker compose logs -f backend
docker compose logs -f frontend
```

## 6. Validate Deployment

Health check backend:

```bash
curl http://localhost:8080/health
```

Open UI:

- `http://<server-ip>:3000`

Login using:

- Email: value from `DEFAULT_ADMIN_EMAIL`
- Password: plaintext password used to generate `DEFAULT_ADMIN_PASSWORD_BCRYPT`

## 7. Production Hardening

### 7.1 Reverse Proxy and TLS (Nginx)

Install Nginx:

```bash
sudo apt-get install -y nginx
```

Example reverse-proxy config:

```nginx
server {
    listen 80;
    server_name your-domain.com;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable TLS with Certbot:

```bash
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d your-domain.com
```

### 7.2 Firewall

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 7.3 Backups

Backup MariaDB volume regularly:

```bash
docker run --rm \
  -v llm_gateway_mariadb_data:/volume \
  -v $(pwd):/backup \
  alpine tar czf /backup/mariadb_data_$(date +%F).tar.gz -C /volume .
```

## 8. Upgrade Procedure

```bash
cd /opt/llm_gateway
git pull
docker compose build --no-cache
docker compose up -d
```

Validate after upgrade:

```bash
curl http://localhost:8080/health
docker compose ps
```

## 9. Troubleshooting

### Backend not starting

```bash
docker compose logs backend
```

Common causes:

- DB credentials mismatch
- Invalid bcrypt hash
- Missing `JWT_SECRET`

### Frontend cannot call API

Check:

- `NEXT_PUBLIC_API_URL`
- `FRONTEND_ORIGIN`
- Reverse proxy path routing for `/api`

### Database migration issues

```bash
docker compose logs db
docker compose logs backend
```

## 10. File References

- Compose stack: [docker-compose.yml](docker-compose.yml)
- Backend image: [backend/Dockerfile](backend/Dockerfile)
- Frontend image: [frontend/Dockerfile](frontend/Dockerfile)
- Environment template: [.env.example](.env.example)
