# 🚀 Quick Start - Observability Stack

## Your Stack is Running! ✅

All observability services are now active:

```
✅ Grafana:    http://localhost:3001 (admin/admin)
✅ Tempo:      http://localhost:3200
✅ Prometheus: http://localhost:19090
✅ Loki:       http://localhost:3100
```

## 🎯 View Your First Trace (3 minutes)

### Step 1: Open Grafana

Open in browser: **http://localhost:3001**

Login: `admin` / `admin`

### Step 2: Go to Explore

1. Click the **compass icon** (🧭) in the left sidebar
2. Or go directly to: http://localhost:3001/explore

### Step 3: Search for Traces

1. Make sure **"Tempo"** is selected in the data source dropdown (top left)
2. Click the **"Search"** tab
3. Click the blue **"Run query"** button
4. You'll see a list of traces!

### Step 4: View Trace Details

1. **Click on any trace** in the list
2. You'll see:
   - 📊 Timeline visualization
   - ⏱️ Duration of each operation
   - 🏷️ HTTP method, status code, path
   - 🔗 Nested spans (if you add custom instrumentation)

## 🧪 Generate More Traces

Run this in your terminal:

```bash
# Make 10 requests to different endpoints
for i in {1..10}; do
  curl -s http://localhost:8080/health > /dev/null
  curl -s http://localhost:8080/api/v1/tunnels/version > /dev/null
  echo "Batch $i sent"
  sleep 0.5
done
```

Then refresh Grafana to see the new traces!

## 📊 What Can You Do Now?

### 1. **View All HTTP Requests**
- Every request to your API is automatically traced
- See timing, status codes, paths
- Identify slow endpoints

### 2. **Debug Performance Issues**
- Find which requests are slow
- See exact timing of operations
- Identify bottlenecks

### 3. **Track Errors**
- See failed requests (4xx, 5xx)
- Get full context of what happened
- Debug issues faster

### 4. **Explore Metrics (Prometheus)**
- Go to: http://localhost:19090
- Try query: `up` to see running services
- View Tempo metrics

### 5. **Search Logs (Loki)**
- In Grafana, go to **Explore**
- Select **"Loki"** data source
- View container logs

## 🎨 Create Your First Dashboard

1. In Grafana, click **"+"** → **"Dashboard"**
2. Click **"Add visualization"**
3. Select **"Tempo"** as data source
4. Build custom views of your traces

## 🔧 Common Tasks

### Start observability stack
```bash
make observability-start
```

### Stop observability stack
```bash
make observability-stop
```

### Check status
```bash
make observability-status
```

### View logs
```bash
docker logs giraffecloud-grafana
docker logs giraffecloud-tempo
```

### Restart everything
```bash
make observability-stop
make observability-start
```

## 🆘 Troubleshooting

### No traces showing up?

1. **Check your server is running and has OTLP endpoint set**:
   ```bash
   # Make sure this is in your .env:
   OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
   ```

2. **Restart your API server** to pick up the env var

3. **Make a request**:
   ```bash
   curl http://localhost:8080/health
   ```

4. **Check server logs** for:
   ```
   [INFO] OpenTelemetry tracing initialized
   ```

### Port conflicts?

If you see "address already in use" errors, we've already fixed the main conflicts:
- Grafana: 3001 (instead of 3000)
- Prometheus: 19090 (instead of 9090)

### Container won't start?

```bash
# Check logs
docker logs giraffecloud-tempo

# Restart everything
docker-compose -f docker-compose.observability.yml down
make observability-start
```

## 📚 Learn More

- **Full Setup Guide**: `OBSERVABILITY_SETUP.md`
- **Production Setup**: `PRODUCTION_SETUP.md`
- **Technical Details**: `observability/README.md`

## 🎉 What's Next?

1. ✅ **You're viewing traces** - Great start!
2. 📈 **Add custom spans** - Instrument your code
3. 📊 **Create dashboards** - Build custom views
4. 🔔 **Set up alerts** - Get notified of issues
5. 🚀 **Deploy to production** - Use `docker-compose.yml`

---

**Enjoying the observability stack?** You now have enterprise-grade monitoring! 🎊


