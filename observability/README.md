# GiraffeCloud Observability Stack

This directory contains the configuration for the complete observability stack using Grafana, Tempo, Prometheus, and Loki.

## 🚀 Quick Start

The observability stack starts automatically when you run:

```bash
make dev
# or
make dev-hot
```

## 📊 Components

| Component | Purpose | URL |
|-----------|---------|-----|
| **Grafana** | Visualization & dashboards | http://localhost:3000 |
| **Tempo** | Distributed tracing | http://localhost:3200 |
| **Prometheus** | Metrics collection | http://localhost:9090 |
| **Loki** | Log aggregation | http://localhost:3100 |

### Default Credentials

- **Grafana**: `admin` / `admin`

## 🎯 Manual Control

Start the stack manually:
```bash
make observability-start
```

Stop the stack:
```bash
make observability-stop
```

Check status:
```bash
make observability-status
```

Or use docker-compose directly:
```bash
docker-compose -f docker-compose.observability.yml up -d
docker-compose -f docker-compose.observability.yml down
docker-compose -f docker-compose.observability.yml ps
```

## 📝 Configuration Files

- `tempo.yaml` - Tempo tracing backend configuration
- `prometheus.yml` - Prometheus metrics scraping configuration
- `grafana/datasources/datasources.yml` - Auto-provisioned data sources
- `grafana/dashboards/dashboard.yml` - Dashboard provisioning config

## 🔍 Using the Stack

### Viewing Traces

1. Open Grafana: http://localhost:3000
2. Go to **Explore** (compass icon in left sidebar)
3. Select **Tempo** as the data source
4. Click **Search** → **Run query**
5. Click on any trace to see detailed spans

### Viewing Metrics

1. Open Grafana: http://localhost:3000
2. Go to **Explore**
3. Select **Prometheus** as the data source
4. Enter a PromQL query (e.g., `up`)

### Viewing Logs

1. Open Grafana: http://localhost:3000
2. Go to **Explore**
3. Select **Loki** as the data source
4. Use LogQL queries to search logs

## 🔗 Correlation Features

The stack is configured to correlate:
- **Traces → Logs**: Click on a trace to see related logs
- **Metrics → Traces**: Click on exemplars in metrics to see traces
- **Logs → Traces**: Click on trace IDs in logs to see full trace

## 📈 Adding Metrics to Your Go App

To expose Prometheus metrics from your Go server:

1. Add the Prometheus client:
```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

2. Add metrics endpoint to your server:
```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

// In your router setup:
router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

3. Uncomment the `giraffecloud-api` job in `prometheus.yml`

4. Restart the observability stack:
```bash
make observability-stop
make observability-start
```

## 🔧 Troubleshooting

### Traces not showing up

1. Check if Tempo is receiving data:
```bash
docker logs giraffecloud-tempo
```

2. Verify your app is sending traces:
```bash
curl http://localhost:3200/api/traces
```

3. Check your `.env` file has:
```
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

### Container won't start

Check logs:
```bash
docker-compose -f docker-compose.observability.yml logs tempo
docker-compose -f docker-compose.observability.yml logs grafana
```

Restart everything:
```bash
make observability-stop
make observability-start
```

## 📚 Resources

- [Grafana Documentation](https://grafana.com/docs/grafana/latest/)
- [Tempo Documentation](https://grafana.com/docs/tempo/latest/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)

## 🗑️ Cleanup

Remove all observability data:
```bash
docker-compose -f docker-compose.observability.yml down -v
```

This will delete all stored traces, metrics, and logs.

