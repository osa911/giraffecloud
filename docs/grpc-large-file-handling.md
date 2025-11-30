### gRPC large file handling – implementation summary

#### Uploads (POST/PUT/PATCH)

- All uploads stream over the existing gRPC tunnel using request-side chunking.
- Protocol: HTTPRequestStart → HTTPRequestChunk(s) → HTTPRequestEnd.
- Server streams request body in chunks; client assembles with io.Pipe and forwards to the local service with a streaming body.
- Router: `routeToGRPCTunnel` directs POST/PUT/PATCH to `ProxyHTTPRequestWithChunking` to avoid 16 MB message limits.

#### Downloads (GET/HEAD)

- Large responses use the existing LargeFileRequest chunked download path (no buffering; io.Pipe).
- Log wording corrected to “routing to gRPC chunked streaming.”

#### Proto and codegen

- `proto/tunnel.proto` extended with:
  - `HTTPRequestStart`, `HTTPRequestChunk`, `HTTPRequestEnd`.
- Generated files live in `internal/tunnel/proto/`:
  - `tunnel.pb.go`
  - `tunnel_grpc.pb.go`
- Re-run protoc only when `proto/tunnel.proto` changes.

#### Logging hygiene

- Late chunks after cleanup are dropped with a debug log to prevent warning floods.
- Broken pipe during client aborts (seek, navigate) is expected and handled.

#### WebSocket recycling

- During intentional WebSocket recycling, the client preserves the gRPC tunnel (no Stop), preventing dropped uploads.
- Only the TCP/WebSocket connection is recycled.

#### Memory & limits

- Memory per request is O(chunkSize) in both directions; no full-file buffering.
- gRPC per-message limit (16 MB) still exists; chunking (default 4 MB) avoids hitting it.
- Chunk size is tunable; 1–4 MB recommended. Larger chunks increase memory/GC/retransmit costs.

#### DoS posture (current & next steps)

- Current: streaming prevents large in-memory allocations; late-chunk dropping avoids buildup.
- Recommended next steps:
  - Per-IP/domain rate limits and quotas.
  - Per-request idle timeouts and overall upload deadlines.
  - Max total upload size per request.
  - Front-proxy limits (Caddy) for body size and rate.

#### Key files

- Router: `internal/tunnel/hybrid_router.go`
- Server streaming (upload/download): `internal/tunnel/grpc_chunked_streaming.go`
- Client streaming: `internal/tunnel/grpc_client.go`
- Proto: `proto/tunnel.proto`
- WebSocket recycle behavior: `internal/tunnel/tunnel.go`
