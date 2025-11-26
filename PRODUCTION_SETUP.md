# 🚀 Production Setup with Grafana Observability

## ✅ What Changed

I've added the **Grafana observability stack** directly into your **production `docker-compose.yml`** file!

Now both development and production environments have:
- 📊 Grafana for visualization
- 🔍 Tempo for distributed tracing
- 📈 Prometheus for metrics
- 📝 Loki for logs

## 📁 File Structure

```
.
├── docker-compose.yml                    # Production (includes observability)
├── docker-compose.observability.yml      # Development only
└── observability/
    ├── tempo.yaml                        # Tempo config
    ├── prometheus.yml                    # Prometheus config
    └── grafana/
        ├── datasources/                  # Auto-configured data sources
        └── dashboards/                   # Dashboard provisioning
```

## 🔧 Why Two Docker Compose Files?

### `docker-compose.yml` (Production)
- **Full stack**: API + Database + Caddy + Observability
- Used for production deployment
- All services on same Docker network
- Services communicate via internal hostnames (e.g., `tempo:4317`)

### `docker-compose.observability.yml` (Development)
- **Only observability**: Grafana, Tempo, Prometheus, Loki
- Used for local development with `make dev`
- Your API runs directly on host (not in container)
- Services exposed to host (e.g., `localhost:4317`)

## 🚀 Production Deployment

### Step 1: Create Production Environment File

```bash
cp internal/config/env/.env.production.example internal/config/env/.env.production
```

Edit the file and set your values:
```bash
# Database credentials
DB_USER=giraffecloud
DB_PASSWORD=your_secure_password_here
DB_NAME=giraffecloud

# Grafana credentials
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=your_secure_password_here

# Your domain
CLIENT_URL=https://yourdomain.com
GRAFANA_ROOT_URL=https://grafana.yourdomain.com
```

### Step 2: Deploy with Docker Compose

```bash
docker-compose up -d
```

This starts:
- ✅ PostgreSQL database
- ✅ Caddy reverse proxy
- ✅ Your API server
- ✅ Tempo (traces)
- ✅ Prometheus (metrics)
- ✅ Loki (logs)
- ✅ Grafana (UI)

### Step 3: Access Grafana

```
http://your-server-ip:3000
```

Login with credentials from your `.env.production` file.

## 🔐 Securing Grafana in Production

### Option 1: Reverse Proxy with Caddy

Add Grafana to your Caddyfile:

```caddy
grafana.yourdomain.com {
    reverse_proxy grafana:3000
}
```

Then:
```bash
docker-compose restart caddy
```

Access: `https://grafana.yourdomain.com`

### Option 2: Change Grafana Port

Edit `docker-compose.yml`:
```yaml
grafana:
  ports:
    - "127.0.0.1:3000:3000"  # Only accessible from localhost
```

Then use SSH tunnel:
```bash
ssh -L 3000:localhost:3000 your-server
```

## 🌐 Network Architecture

In production, services communicate via Docker's internal network:

```
┌─────────────────────────────────────────────┐
│         giraffecloud_network                │
│         (172.20.0.0/16)                     │
│                                             │
│  ┌──────────┐      ┌──────────┐            │
│  │   API    │─────▶│  Tempo   │            │
│  │172.20.0.2│      │172.20.0.5│            │
│  └──────────┘      └──────────┘            │
│       │                  │                  │
│       │            ┌──────────┐            │
│       └───────────▶│ Postgres │            │
│                    │172.20.0.3│            │
│                    └──────────┘            │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐ │
│  │Prometheus│  │  Loki    │  │ Grafana  │ │
│  │172.20.0.6│  │172.20.0.7│  │172.20.0.8│ │
│  └──────────┘  └──────────┘  └──────────┘ │
│                                             │
│  ┌──────────┐                               │
│  │  Caddy   │ (reverse proxy)               │
│  │172.20.0.4│                               │
│  └──────────┘                               │
└─────────────────────────────────────────────┘
         │
         ▼
    Internet
```

## 🔍 Key Configuration Details

### 1. OTLP Endpoint Override

In `docker-compose.yml`, the API service has:
```yaml
environment:
  - OTEL_EXPORTER_OTLP_ENDPOINT=tempo:4317
```

This **overrides** any `.env` file setting, ensuring the API always connects to Tempo via internal network.

### 2. Service Dependencies

```yaml
api:
  depends_on:
    - postgres
    - tempo
```

Ensures Tempo starts before the API server.

### 3. Persistent Storage

All data is stored in Docker volumes:
- `postgres_data` - Database
- `tempo_data` - Traces
- `prometheus_data` - Metrics
- `loki_data` - Logs
- `grafana_data` - Dashboards & settings

## 📊 Monitoring Your Production App

### View Traces
1. Open Grafana: `http://your-server:3000`
2. Go to **Explore** → **Tempo**
3. Search for traces
4. See all API requests, timing, errors

### View Metrics
1. Go to **Explore** → **Prometheus**
2. Query metrics (when you add `/metrics` endpoint)

### View Logs
1. Go to **Explore** → **Loki**
2. Search container logs
3. Filter by service

## 🆚 Development vs Production

| Aspect | Development | Production |
|--------|-------------|------------|
| **Docker Compose File** | `docker-compose.observability.yml` | `docker-compose.yml` |
| **API Runs In** | Host machine | Docker container |
| **OTLP Endpoint** | `localhost:4317` | `tempo:4317` |
| **Start Command** | `make dev` | `docker-compose up -d` |
| **Grafana Auth** | Anonymous enabled | Username/password |
| **Network** | Separate | Same network |

## 🔧 Useful Production Commands

### View all service logs
```bash
docker-compose logs -f
```

### View specific service logs
```bash
docker-compose logs -f api
docker-compose logs -f tempo
docker-compose logs -f grafana
```

### Restart a service
```bash
docker-compose restart api
docker-compose restart grafana
```

### Check service status
```bash
docker-compose ps
```

### Stop everything
```bash
docker-compose down
```

### Remove all data (⚠️ destructive)
```bash
docker-compose down -v
```

## 📈 Next Steps

1. ✅ **Deploy to production** - `docker-compose up -d`
2. 🔐 **Secure Grafana** - Set up reverse proxy or firewall
3. 📊 **Create dashboards** - Build custom views in Grafana
4. 📈 **Add metrics endpoint** - Expose `/metrics` from your Go app
5. 🔔 **Set up alerts** - Get notified when things go wrong
6. 📝 **Ship logs to Loki** - Forward application logs

## 🆘 Troubleshooting

### Services won't start
```bash
docker-compose down
docker-compose up -d
docker-compose logs
```

### Tempo not receiving traces
```bash
# Check Tempo logs
docker-compose logs tempo

# Check API can reach Tempo
docker-compose exec api ping tempo

# Verify OTLP endpoint
docker-compose exec api env | grep OTEL
```

### Port conflicts
If ports are already in use, edit `docker-compose.yml`:
```yaml
ports:
  - "3001:3000"  # Change external port
```

## 📚 Documentation

- Production setup: This file
- Development setup: `OBSERVABILITY_SETUP.md`
- Observability details: `observability/README.md`

---

**You're all set for production!** 🚀

Both development and production now have full observability with Grafana, Tempo, Prometheus, and Loki.

