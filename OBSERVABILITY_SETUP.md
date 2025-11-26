# 🎯 Grafana Observability Stack - Setup Complete!

## ✅ What Was Set Up

1. **Docker Compose** - Full observability stack with:

   - 📊 **Grafana** - Visualization (Port 3000)
   - 🔍 **Tempo** - Distributed Tracing (Port 4317, 3200)
   - 📈 **Prometheus** - Metrics (Port 9090)
   - 📝 **Loki** - Logs (Port 3100)

2. **Automatic Startup** - Stack starts automatically with `make dev`

3. **Configuration Files**:
   - `docker-compose.observability.yml` - Main compose file
   - `observability/tempo.yaml` - Tempo config
   - `observability/prometheus.yml` - Prometheus config
   - `observability/grafana/datasources/` - Auto-provisioned data sources

## 🚀 Getting Started

### Step 1: Update Your .env File

Add this to your `.env` file (or create it if it doesn't exist):

```bash
# Telemetry Configuration - Grafana Stack
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
```

**Note**: The endpoint is the same as Jaeger, but now it's pointing to Tempo!

### Step 2: Start Everything

```bash
make dev
```

This will:

1. ✅ Start the observability stack (Grafana, Tempo, Prometheus, Loki)
2. ✅ Start Caddy
3. ✅ Start your API server

### Step 3: Access Grafana

Open in your browser:

```
http://localhost:3001
```

**Login**: `admin` / `admin`

> **Note**: Port 3001 is used instead of 3000 due to port conflicts with other applications on your system.

### Step 4: View Your Traces

1. In Grafana, click **Explore** (compass icon) in the left sidebar
2. Select **Tempo** from the dropdown
3. Click **Search** → **Run query**
4. Make some API requests:
   ```bash
   curl http://localhost:8080/health
   curl http://localhost:8080/api/v1/tunnels/version
   ```
5. Refresh the search - you'll see your traces!
6. Click on any trace to see detailed timing

## 🎮 Manual Control Commands

Start observability stack:

```bash
make observability-start
```

Stop observability stack:

```bash
make observability-stop
```

Check status:

```bash
make observability-status
```

## 📊 What You Can Do Now

### 1. View Traces (What You Already Know)

- See request flow through your application
- Identify slow operations
- Debug errors

### 2. View Metrics (NEW!)

- Go to **Explore** → Select **Prometheus**
- Try queries like:
  - `up` - See which services are running
  - `tempo_ingester_traces_received_total` - Traces received

### 3. View Logs (NEW!)

- Go to **Explore** → Select **Loki**
- Search logs from all containers
- Filter by service, level, etc.

### 4. Correlate Everything (POWERFUL!)

- Click on a trace → See related logs
- Click on metrics → Jump to traces
- All connected in one place!

## 🆚 Jaeger vs Grafana - What Changed?

| Feature     | Jaeger (Before) | Grafana (Now) |
| ----------- | --------------- | ------------- |
| Traces      | ✅              | ✅            |
| Metrics     | ❌              | ✅            |
| Logs        | ❌              | ✅            |
| Correlation | ❌              | ✅            |
| Dashboards  | ❌              | ✅            |
| Auto-start  | ❌              | ✅            |

**The OTLP endpoint stays the same** - your code doesn't need to change!

## 🔧 Troubleshooting

### Traces not showing up?

1. Check server logs for:

   ```
   [INFO] Initializing OpenTelemetry tracing...
   [INFO] OpenTelemetry tracing initialized
   ```

2. Check Tempo is running:

   ```bash
   docker ps | grep tempo
   ```

3. Check Tempo logs:
   ```bash
   docker logs giraffecloud-tempo
   ```

### Port conflicts?

If ports are already in use, edit `docker-compose.observability.yml`:

```yaml
ports:
  - "3001:3000" # Change Grafana to port 3001
```

### Want to reset everything?

```bash
docker-compose -f docker-compose.observability.yml down -v
make observability-start
```

## 📚 Next Steps

1. ✅ **You're Done!** - Everything starts automatically now
2. 📊 **Create Dashboards** - Build custom dashboards in Grafana
3. 📈 **Add Metrics** - Expose `/metrics` endpoint from your Go app
4. 🔔 **Set Alerts** - Get notified when things go wrong
5. 📝 **Ship Logs** - Send application logs to Loki

## 💡 Pro Tips

1. **Bookmark Grafana**: http://localhost:3000
2. **Explore First**: Learn by clicking around in Explore view
3. **Create Dashboards**: Once comfortable, build dashboards
4. **Use Search**: Tempo search is powerful - use it!
5. **Check the Docs**: See `observability/README.md` for details

## 🎉 You're All Set!

Your observability stack is now:

- ✅ Configured
- ✅ Running automatically with your server
- ✅ Ready to use
- ✅ More powerful than Jaeger

Just run `make dev` and everything starts together!

---

**Questions?** Check `observability/README.md` for detailed documentation.
