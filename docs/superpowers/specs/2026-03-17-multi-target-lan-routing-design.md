# Multi-Target LAN Routing Design

**Date:** 2026-03-17
**Status:** Approved

## Problem

GiraffeCloud currently uses a 1:1 model: one CLI instance connects to one domain and proxies to one local port on localhost. To expose multiple services, users must run multiple CLI instances and configure each separately. Users with services on different machines in their LAN (e.g., a NAS at 192.168.1.5, a Raspberry Pi at 192.168.1.10) must install the CLI on each machine.

## Goal

Evolve to a Cloudflare Tunnel-style model: one CLI instance acts as a LAN gateway, serving all of a user's tunnels. Each tunnel maps a domain to a `target_host:target_port` in the local network. All configuration is managed from the web dashboard and pushed to the CLI in real-time.

## Approach: Evolve the Handshake

Extend the existing gRPC bidirectional stream and control channel. No new RPCs, no new entities, no polling.

---

## 1. Database Changes

### Add `target_host` to Tunnel schema

```go
// internal/db/ent/schema/tunnel.go
field.String("target_host").
    Default("localhost").
    StructTag(`json:"target_host"`),
```

### Uniqueness constraint (DB-level)

Add a composite unique index in Ent to enforce uniqueness at the database level:

```go
func (Tunnel) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("user_id", "target_host", "target_port").Unique(),
    }
}
```

This prevents race conditions that application-level-only checks cannot catch. The application layer still validates for better error messages, but the DB index is the authoritative constraint.

### Per-tunnel `token` field

The existing per-tunnel `token` field (used in the old single-tunnel handshake) becomes unused. It will be removed from the schema. The user's API token (from the `tokens` table) is the sole authentication mechanism for the gRPC handshake.

### Migration

Add `target_host` column with default `"localhost"`. Drop the per-tunnel `token` column. All existing tunnels work unchanged.

---

## 2. Protocol (Proto) Changes

### New message for tunnel routing config

```protobuf
message TunnelRouteConfig {
    string domain = 1;
    string target_host = 2;
    int32 target_port = 3;
    bool is_enabled = 4;
}
```

### Extend handshake response

The current handshake response is a `TunnelControl` containing a `TunnelStatus` inside a `TunnelMessage`. Add a `repeated TunnelRouteConfig routes` field to `TunnelStatus`. The server populates this with all enabled tunnels for the authenticated user when sending the handshake response.

```protobuf
message TunnelStatus {
    // ... existing fields ...
    repeated TunnelRouteConfig routes = 10;  // All user's enabled tunnels
}
```

### Add ConfigUpdate control message

```protobuf
enum ConfigUpdateReason {
    CONFIG_UPDATE_REASON_UNSPECIFIED = 0;
    CONFIG_UPDATE_REASON_TUNNEL_CREATED = 1;
    CONFIG_UPDATE_REASON_TUNNEL_UPDATED = 2;
    CONFIG_UPDATE_REASON_TUNNEL_DELETED = 3;
}

message ConfigUpdate {
    repeated TunnelRouteConfig routes = 1;  // Full replacement of routing table
    ConfigUpdateReason reason = 2;
}
```

Add `ConfigUpdate config_update` to the `ControlMessage` oneof.

### Control channel handshake change

The current `ControlHandshake` identifies the stream by `domain`. Change to identify by `token` (the user's API token) instead. The server looks up the active `TunnelStream` by user ID (derived from the token) rather than by domain.

```protobuf
message ControlHandshake {
    string token = 1;   // User's API token (replaces domain-based lookup)
    string domain = 2;  // Deprecated, ignored
}
```

### ConfigUpdate semantics

ConfigUpdate is **best-effort**: the server pushes the update over the control channel. If the stream is temporarily unavailable, the update is dropped — the CLI will get the correct state on next reconnect via the handshake. No acknowledgment required from the CLI.

### Handshake flow

1. CLI sends handshake with `token` only (no domain/port)
2. Server validates token → gets user ID → fetches all enabled tunnels
3. Server responds with `TunnelStatus` containing `routes` list
4. Dashboard changes trigger `ConfigUpdate` push over control channel
5. CLI updates routing table in-memory — no reconnect needed

---

## 3. Server-Side Changes

### Auth flow restructuring (`auth_helpers.go`)

`AuthenticateTunnelByToken` currently returns a single `*ent.Tunnel` and errors on multiple tunnels. Change to return `[]*ent.Tunnel` (all enabled tunnels for the user). The error case for "multiple enabled tunnels found" is removed — that's now the expected case.

### gRPC handshake (`grpc_service.go`)

- `EstablishTunnel` receives handshake with `token`, calls updated `AuthenticateTunnelByToken` to get all tunnels
- Responds with `TunnelStatus` containing full `routes` list
- Registers the gRPC stream for ALL user domains in `tunnelStreams` map
- Multiple domain keys point to the same `*TunnelStream`
- Store a reverse mapping: `TunnelStream` holds a `userID` and list of `domains` for cleanup

### Stream cleanup on disconnect

When a `TunnelStream` disconnects, the cleanup logic must:
1. Iterate all domains associated with that stream (stored on the `TunnelStream` struct)
2. Delete each domain key from the `tunnelStreams` map
3. Call `UpdateClientIP(ctx, tunnelID, "")` for each tunnel to clear client IP
4. Remove Caddy routes for all associated domains

```go
// On TunnelStream, add:
type TunnelStream struct {
    // ... existing fields ...
    UserID  uint32
    Domains []string  // All domains served by this stream
}
```

### Control channel identity

The `ControlChannel` RPC currently binds to a `TunnelStream` by domain lookup. Change to use token-based lookup:
1. Client sends `ControlHandshake{token: "..."}`
2. Server validates token → gets user ID
3. Server finds the active `TunnelStream` by user ID (new index: `userStreams map[uint32]*TunnelStream`)
4. Binds control stream to that `TunnelStream`

### Caddy integration

On multi-domain connect:
- Call `UpdateClientIP` for each tunnel, triggering `CaddyService.ConfigureRoute` for each domain

On disconnect:
- Call `UpdateClientIP(ctx, tunnelID, "")` for each tunnel, triggering Caddy route removal

On `ConfigUpdate` push (new tunnel added while connected):
- Configure Caddy route for the new domain
- On tunnel deleted: remove Caddy route for that domain

### Config push on dashboard changes

When `TunnelService.CreateTunnel/UpdateTunnel/DeleteTunnel` is called:
- Check if the user has an active gRPC stream (via `userStreams` map)
- If yes, push `ConfigUpdate` with full updated routes list over the control channel
- Update the `tunnelStreams` map (add/remove domain entries)

### TunnelStatusCache

The existing `TunnelStatusCache` remains functional — it's keyed by domain, and `statusCache.IsEnabled(domain)` still works. No changes needed; it continues to query all enabled tunnels and cache per-domain status.

### Tunnel service validation

- Remove old duplicate-port check
- Add new uniqueness check: `target_host + target_port` per user (application-level for good error messages, backed by DB composite index)
- `target_host` supports IP addresses and hostnames (e.g., `192.168.1.5`, `my-nas.local`)
- No protocol prefix, no path — just host/IP

---

## 4. CLI Changes

### Routing table

```go
type RouteTable struct {
    mu     sync.RWMutex
    routes map[string]*Route  // domain -> target
}

type Route struct {
    Domain     string
    TargetHost string
    TargetPort int32
}

func (rt *RouteTable) Resolve(domain string) *Route
func (rt *RouteTable) Update(routes []*proto.TunnelRouteConfig)
```

### Connection changes

- `Connect()` takes only `token` — no `domain` or `localPort`
- On handshake response, populate routing table from `routes`
- When proxying a request, resolve domain → `Route`, then `net.Dial("tcp", route.TargetHost:route.TargetPort)`
- Handle `ConfigUpdate` control messages by calling `routeTable.Update()`

### Empty routes handling

If the server returns zero routes (no enabled tunnels), the CLI stays connected with an empty routing table. It logs a message: "No tunnels configured. Create tunnels at https://giraffecloud.xyz/dashboard/tunnels". When a `ConfigUpdate` arrives with routes, the CLI starts serving immediately.

### CLI UX

- `giraffecloud connect` — no flags needed
- Remove `--domain` flag
- Output on connect:
  ```
  Connected to GiraffeCloud
  Serving 3 tunnels:
    app.example.com -> 192.168.1.5:8080
    api.example.com -> localhost:3000
    nas.example.com -> 192.168.1.10:5000
  ```
- Live updates logged when config changes arrive:
  ```
  Tunnel added: blog.example.com -> 192.168.1.20:4000
  Now serving 4 tunnels
  ```

### Config simplification

`Domain` and `LocalPort` fields in config.json become unused. Kept in struct to avoid breaking JSON deserialization of existing configs, but ignored. The `Validate()` method does not currently require these fields, so no changes needed there.

---

## 5. Dashboard (Frontend) Changes

### Tunnel Dialog (`TunnelDialog.tsx`)

- Add "Target Host" input field alongside "Port"
- Default value: `localhost`
- Validation: valid IP address or hostname (no protocol prefix, no path)
- Help text: "IP or hostname of the machine in your local network (e.g., 192.168.1.5, my-nas.local)"

### Tunnel List (`TunnelList.tsx`)

- Add "Target" column showing `target_host:target_port`
- When `target_host` is `localhost`, show just the port for cleanliness
- Otherwise show full `host:port`

### Types (`types/tunnel.ts`)

Add `target_host: string` to the `Tunnel` interface.

---

## 6. Documentation & Getting Started

### Getting Started page (`GettingStartedPage.tsx`)

Update steps:
1. Create API token (unchanged)
2. Login with token (unchanged)
3. Create tunnels in the dashboard — configure domain → target_host:port mappings
4. Connect — just `giraffecloud connect` (no flags)
5. New section: "Exposing LAN services" — one CLI, multiple machines

### CLI help text

- Update `connect` command description to reflect multi-tunnel mode
- Remove `--domain` flag documentation
- Update examples

### Docs

- `installation.md` — update quick start
- `hybrid-tunnel-architecture.md` — document multi-target routing
- New section: "LAN routing" use case
- `README.md` — highlight LAN routing as a feature

---

## 7. No Backward Compatibility

Clean break. No old-style handshake detection. Deploy everything at once:
1. Deploy server + dashboard + new CLI binaries
2. Set minimum required version via admin dashboard
3. Old CLIs hit forced update → auto-update → reconnect with new protocol

The existing auto-update mechanism and forced update flag handle the transition.

---

## Summary of File Changes

### Backend (Go)
- `internal/db/ent/schema/tunnel.go` — add `target_host` field, composite index, remove `token` field
- `proto/tunnel.proto` — add `TunnelRouteConfig`, `ConfigUpdate`, `ConfigUpdateReason`, extend `TunnelStatus` with routes, update `ControlHandshake`
- `internal/tunnel/auth_helpers.go` — return `[]*ent.Tunnel` instead of single tunnel
- `internal/tunnel/grpc_service.go` — multi-domain stream registration, `userStreams` map, shared-stream cleanup, config push, Caddy multi-domain setup/teardown
- `internal/tunnel/grpc_client.go` — routing table, multi-target proxy, ConfigUpdate handler
- `internal/tunnel/tunnel.go` — routing table integration, proxy to target_host
- `internal/service/tunnel_service.go` — new uniqueness validation (host+port per user), config push trigger
- `internal/repository/tunnel_repository.go` — add `TargetHost` to `TunnelUpdate` struct
- `internal/api/handlers/tunnel.go` — accept target_host in create/update
- `internal/api/dto/v1/tunnel/dto.go` — add target_host to DTOs
- `internal/api/mapper/tunnel_mapper.go` — map target_host
- `cmd/giraffecloud/main.go` — simplify connect command, remove --domain flag

### Frontend (TypeScript)
- `apps/web/src/types/tunnel.ts` — add target_host
- `apps/web/src/components/dashboard/tunnels/TunnelDialog.tsx` — target host input
- `apps/web/src/components/dashboard/tunnels/TunnelList.tsx` — target column
- `apps/web/src/components/dashboard/GettingStartedPage.tsx` — updated flow

### Proto
- `proto/tunnel.proto` — new messages, enums, and fields

### Docs
- `docs/installation.md`
- `docs/hybrid-tunnel-architecture.md`
- `README.md`
