# Multi-Target LAN Routing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Evolve GiraffeCloud from one-CLI-one-tunnel to Cloudflare Tunnel-style multi-target LAN routing where one CLI instance serves all user tunnels, each mapping a domain to a target_host:port in the local network.

**Architecture:** Add `target_host` field to existing Tunnel entity (default "localhost"). Extend gRPC handshake to return all user tunnels. CLI builds a routing table and proxies each domain to its target. Dashboard changes push in real-time via control channel ConfigUpdate messages.

**Tech Stack:** Go 1.24, gRPC/Protobuf, Ent ORM, PostgreSQL, Next.js 16, React 19, Tailwind 4, shadcn/ui

**Spec:** `docs/superpowers/specs/2026-03-17-multi-target-lan-routing-design.md`

---

## Chunk 1: Database & API Layer

### Task 1: Add `target_host` to Tunnel Schema

**Files:**
- Modify: `internal/db/ent/schema/tunnel.go`

NOTE: Do NOT remove the `token` field in this task. Token removal happens in Task 6 after all dependent code is updated.

- [ ] **Step 1: Add `target_host` field and composite index**

In `Fields()`, add after the `domain` field (after line 19):

```go
field.String("target_host").
    Default("localhost").
    StructTag(`json:"target_host"`),
```

Add a new `Indexes()` method to the Tunnel struct:

```go
func (Tunnel) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("user_id", "target_host", "target_port").Unique(),
    }
}
```

Add `"entgo.io/ent/schema/index"` to imports.

- [ ] **Step 2: Regenerate Ent code**

Run: `go generate ./internal/db/ent`
Expected: Ent generates updated client code with `target_host` field and composite index.

- [ ] **Step 3: Commit**

```bash
git add internal/db/ent/
git commit -m "feat: add target_host field to tunnel schema with composite index"
```

---

### Task 2: Update Tunnel Repository

**Files:**
- Modify: `internal/repository/tunnel_repository.go`

- [ ] **Step 1: Add `TargetHost` to `TunnelUpdate` struct**

The current struct at lines 96-100 is:
```go
type TunnelUpdate struct {
    IsEnabled            *bool
    TargetPort           *int
    DnsPropagationStatus *tunnel.DNSPropagationStatus
}
```

Add `TargetHost` field:
```go
type TunnelUpdate struct {
    TargetHost           *string
    IsEnabled            *bool
    TargetPort           *int
    DnsPropagationStatus *tunnel.DNSPropagationStatus
}
```

- [ ] **Step 2: Update the `Update()` method to handle `TargetHost`**

In the `Update()` method (lines 103-128), add after the existing field checks (around line 115):
```go
if u.TargetHost != nil {
    update.SetTargetHost(*u.TargetHost)
}
```

- [ ] **Step 3: Update the `Create()` method to set `target_host`**

In `Create()` (lines 34-42), add `.SetTargetHost(t.TargetHost)` to the Ent builder chain after `.SetDomain(t.Domain)`.

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/repository/...`

- [ ] **Step 5: Commit**

```bash
git add internal/repository/tunnel_repository.go
git commit -m "feat: add target_host to tunnel repository"
```

---

### Task 3: Update Tunnel DTOs and Mapper

**Files:**
- Modify: `internal/api/dto/v1/tunnel/dto.go`
- Modify: `internal/api/mapper/tunnel_mapper.go`

- [ ] **Step 1: Add `target_host` to tunnel DTOs**

The current DTOs use `time.Time` for timestamps — preserve this. Only add the `TargetHost` field.

In `CreateRequest` (lines 6-9), add `TargetHost`:
```go
type CreateRequest struct {
    Domain     string `json:"domain"`
    TargetHost string `json:"target_host"`
    TargetPort int    `json:"target_port" binding:"required,min=1,max=65535"`
}
```

In `CreateResponse` (lines 12-20), add `TargetHost` after `Domain`:
```go
TargetHost string    `json:"target_host"`
```

In `Response` (lines 23-31), add `TargetHost` after `Domain`:
```go
TargetHost string    `json:"target_host"`
```

In `UpdateRequest` (lines 34-37), add `TargetHost`:
```go
type UpdateRequest struct {
    TargetHost *string `json:"target_host,omitempty"`
    IsEnabled  *bool   `json:"is_enabled,omitempty"`
    TargetPort *int    `json:"target_port,omitempty" binding:"omitempty,min=1,max=65535"`
}
```

- [ ] **Step 2: Update mapper functions**

In `tunnel_mapper.go`, add `TargetHost: t.TargetHost` to:
- `TunnelToCreateResponse()` (line 14-22) — add after `Domain: t.Domain,`
- `TunnelToResponse()` (line 31-39) — add after `Domain: t.Domain,`

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/api/...`

- [ ] **Step 4: Commit**

```bash
git add internal/api/dto/v1/tunnel/dto.go internal/api/mapper/tunnel_mapper.go
git commit -m "feat: add target_host to tunnel DTOs and mapper"
```

---

### Task 4: Update Tunnel Service Validation

**Files:**
- Modify: `internal/service/tunnel_service.go`
- Modify: `internal/interfaces/tunnel_service.go`

- [ ] **Step 1: Update `CreateTunnel` signature**

Current signature at line 99:
```go
func (s *tunnelService) CreateTunnel(ctx context.Context, userID uint32, domain string, targetPort int) (*ent.Tunnel, error) {
```

Change to:
```go
func (s *tunnelService) CreateTunnel(ctx context.Context, userID uint32, domain string, targetHost string, targetPort int) (*ent.Tunnel, error) {
```

Add at the beginning of the function:
```go
if targetHost == "" {
    targetHost = "localhost"
}
```

- [ ] **Step 2: Replace duplicate-port check with host+port uniqueness**

In `CreateTunnel()`, replace the port-only check (around lines 113-119):
```go
// OLD
for _, tunnel := range tunnels {
    if tunnel.TargetPort == targetPort {
```

With:
```go
// NEW
for _, t := range tunnels {
    if t.TargetHost == targetHost && t.TargetPort == targetPort {
        logger.Warn("User %d attempted to create tunnel with duplicate target %s:%d", userID, targetHost, targetPort)
        return nil, fmt.Errorf("%w: you already have a tunnel targeting %s:%d", ErrConflict, targetHost, targetPort)
    }
}
```

- [ ] **Step 3: Add `TargetHost` to tunnel entity creation**

In the existing struct literal at line 174, add `TargetHost: targetHost,` alongside the existing fields.

- [ ] **Step 4: Add `target_host` validation**

Add validation to reject invalid target_host values (no protocol prefix, no path):
```go
if strings.Contains(targetHost, "://") || strings.Contains(targetHost, "/") {
    return nil, fmt.Errorf("%w: target_host must be a hostname or IP address without protocol or path", ErrValidation)
}
```

- [ ] **Step 5: Update `UpdateTunnel` host+port validation**

In `UpdateTunnel()` (around line 241), replace the port-only uniqueness check with a combined host+port check:

```go
if u.TargetPort != nil || u.TargetHost != nil {
    newPort := currentTunnel.TargetPort
    newHost := currentTunnel.TargetHost
    if u.TargetPort != nil {
        newPort = *u.TargetPort
    }
    if u.TargetHost != nil {
        newHost = *u.TargetHost
    }
    if newPort != currentTunnel.TargetPort || newHost != currentTunnel.TargetHost {
        tunnels, err := s.repo.GetByUserID(ctx, userID)
        if err != nil {
            return nil, fmt.Errorf("failed to check existing tunnels: %w", err)
        }
        for _, t := range tunnels {
            if t.ID != int(tunnelID) && t.TargetHost == newHost && t.TargetPort == newPort {
                return nil, fmt.Errorf("%w: you already have a tunnel targeting %s:%d", ErrConflict, newHost, newPort)
            }
        }
    }
}
```

- [ ] **Step 6: Update the TunnelService interface**

In `internal/interfaces/tunnel_service.go`, update:
```go
CreateTunnel(ctx context.Context, userID uint32, domain string, targetHost string, targetPort int) (*ent.Tunnel, error)
```

- [ ] **Step 7: Verify compilation**

Run: `go build ./internal/service/... ./internal/interfaces/...`

- [ ] **Step 8: Commit**

```bash
git add internal/service/tunnel_service.go internal/interfaces/tunnel_service.go
git commit -m "feat: update tunnel service with target_host validation"
```

---

### Task 5: Update Tunnel Handler

**Files:**
- Modify: `internal/api/handlers/tunnel.go`

- [ ] **Step 1: Update `CreateTunnel` handler**

In `CreateTunnel()` (around line 136), update the service call to pass `target_host`:

```go
targetHost := req.TargetHost
if targetHost == "" {
    targetHost = "localhost"
}
tunnel, err := h.tunnelService.CreateTunnel(c.Request.Context(), userID, req.Domain, targetHost, req.TargetPort)
```

- [ ] **Step 2: Update `UpdateTunnel` handler**

In `UpdateTunnel()` (around lines 236-239), the current code builds a `TunnelUpdate`. Add `TargetHost` alongside the existing fields:

```go
update := &repository.TunnelUpdate{
    TargetHost: req.TargetHost,
    IsEnabled:  req.IsEnabled,
    TargetPort: req.TargetPort,
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/api/handlers/...`

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers/tunnel.go
git commit -m "feat: update tunnel handler to accept target_host"
```

---

### Task 6: Remove per-tunnel token references

**Files:**
- Modify: `internal/db/ent/schema/tunnel.go` — remove `token` field
- Modify: `internal/service/tunnel_service.go` — remove `generateToken()` call
- Modify: `internal/api/dto/v1/tunnel/dto.go` — remove `Token` from `CreateResponse`
- Modify: `internal/api/mapper/tunnel_mapper.go` — remove `Token` mapping
- Modify: `internal/repository/tunnel_repository.go` — remove `.SetToken()` in `Create()`, remove `GetByToken()` method
- Modify: `internal/interfaces/tunnel_service.go` — remove `GetByToken` from interface if present

- [ ] **Step 1: Check all usages of tunnel-level token**

Run: `grep -rn "\.Token\b" internal/ --include="*.go" | grep -i tunnel`
Run: `grep -rn "GetByToken" internal/ --include="*.go"`
Run: `grep -rn "generateToken" internal/ --include="*.go"`

Identify every reference that needs updating.

- [ ] **Step 2: Remove `token` field from schema**

In `internal/db/ent/schema/tunnel.go`, remove the token field:
```go
field.String("token").
    NotEmpty().
    Unique(),
```

- [ ] **Step 3: Regenerate Ent code**

Run: `go generate ./internal/db/ent`

- [ ] **Step 4: Remove all token references from service, handler, mapper, repository, DTOs**

Remove `generateToken()` function and its call in `tunnel_service.go`.
Remove `Token` field from `CreateResponse` in DTO.
Remove `Token: t.Token` from mapper.
Remove `.SetToken(t.Token)` from repository `Create()`.
Remove `GetByToken()` method from repository (and interface if present).
Note: The `GetByToken` at `tunnel_repository.go:16` is in the `TunnelRepository` interface — remove it. Also remove implementation at lines 67-78.

- [ ] **Step 5: Check TCP tunnel server for token references**

Run: `grep -rn "GetByToken\|\.Token" internal/tunnel/server.go --include="*.go"`

The TCP tunnel server at `internal/tunnel/server.go:182` calls `AuthenticateTunnelByToken` which uses the API token (from `tokenRepo`), not the tunnel-level token. So this should be fine. Verify.

- [ ] **Step 6: Verify full compilation**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/
git commit -m "refactor: remove per-tunnel token field from schema and all references"
```

---

## Chunk 2: Protocol & gRPC Server Changes

### Task 7: Update Protobuf Definitions

**Files:**
- Modify: `proto/tunnel.proto`

- [ ] **Step 1: Add TunnelRouteConfig message**

Add after existing message definitions:

```protobuf
message TunnelRouteConfig {
    string domain = 1;
    string target_host = 2;
    int32 target_port = 3;
    bool is_enabled = 4;
}
```

- [ ] **Step 2: Add ConfigUpdate message and enum**

```protobuf
enum ConfigUpdateReason {
    CONFIG_UPDATE_REASON_UNSPECIFIED = 0;
    CONFIG_UPDATE_REASON_TUNNEL_CREATED = 1;
    CONFIG_UPDATE_REASON_TUNNEL_UPDATED = 2;
    CONFIG_UPDATE_REASON_TUNNEL_DELETED = 3;
}

message ConfigUpdate {
    repeated TunnelRouteConfig routes = 1;
    ConfigUpdateReason reason = 2;
}
```

- [ ] **Step 3: Extend TunnelStatus with routes**

In the `TunnelStatus` message (around line 214-222), add a new field with a high field number to avoid conflicts:

```protobuf
repeated TunnelRouteConfig routes = 10;
```

The existing single-domain fields (`domain`, `target_port`) remain for wire compat but the server will populate `routes` instead.

- [ ] **Step 4: Add ConfigUpdate to ControlMessage oneof**

In the `ControlMessage` message, add to the oneof with a high field number:

```protobuf
ConfigUpdate config_update = 10;
```

- [ ] **Step 5: Update ControlHandshake for token-based identity**

Check current `ControlHandshake` field numbers first. The current message likely has `domain = 1`. Since this is a clean break (no backward compat), we can reassign:

```protobuf
message ControlHandshake {
    string token = 1;
    string domain = 2;  // Deprecated, kept for field number stability
}
```

If `domain` is already field 1, add `token` as a NEW field number (e.g., `string token = 3`) and keep `domain = 1` but ignore it server-side. This avoids wire-format confusion.

- [ ] **Step 6: Deprecate `TunnelHandshake.domain` and `TunnelHandshake.target_port`**

The `TunnelHandshake` message currently has `token = 1, domain = 2, target_port = 3`. In the new model, the client only sends `token`. Keep the field numbers but document that `domain` and `target_port` are ignored by the server.

- [ ] **Step 7: Regenerate protobuf Go code**

Run: `make proto` (check `makefiles/proto.mk` for the exact command)
Expected: Generated `.pb.go` and `_grpc.pb.go` files updated.

- [ ] **Step 8: Commit**

```bash
git add proto/ internal/tunnel/proto/
git commit -m "feat: add multi-tunnel routing messages to protobuf"
```

---

### Task 8: Update Auth Helpers for Multi-Tunnel

**Files:**
- Modify: `internal/tunnel/auth_helpers.go`
- Modify: `internal/tunnel/grpc_service_helpers.go`

- [ ] **Step 1: Change `AuthenticateTunnelByToken` to return all tunnels**

Current signature at `auth_helpers.go:13`:
```go
func AuthenticateTunnelByToken(ctx, token, domain string, tokenRepo, tunnelRepo) (*ent.Tunnel, error)
```

Change to:
```go
func AuthenticateTunnelByToken(ctx context.Context, token string, tokenRepo repository.TokenRepository, tunnelRepo repository.TunnelRepository) ([]*ent.Tunnel, uint32, error)
```

Returns `([]*ent.Tunnel, userID, error)`. The `domain` param is removed. The `userID` is returned because callers need it for stream registration.

New logic:
1. Validate API token → get `apiToken.UserID`
2. Fetch all tunnels for user via `tunnelRepo.GetByUserID`
3. Filter for enabled tunnels
4. Return enabled tunnels + userID (empty list is valid, not an error)
5. Remove all domain-matching logic and "multiple tunnels" error

- [ ] **Step 2: Update `authenticateTunnel` wrapper in `grpc_service_helpers.go`**

At `grpc_service_helpers.go:90`, the wrapper calls `AuthenticateTunnelByToken` with `handshake.Domain`. Update to:

```go
func (s *GRPCTunnelServer) authenticateTunnel(ctx context.Context, handshake *proto.TunnelHandshake) ([]*ent.Tunnel, uint32, error) {
    tunnels, userID, err := AuthenticateTunnelByToken(ctx, handshake.Token, s.tokenRepo, s.tunnelRepo)
    if err != nil {
        return nil, 0, err
    }
    s.logger.Info("[gRPC AUTH] Authenticated user %d with %d enabled tunnels", userID, len(tunnels))
    return tunnels, userID, nil
}
```

- [ ] **Step 3: Update TCP tunnel server caller**

At `internal/tunnel/server.go:182`, `AuthenticateTunnelByToken` is called with `req.Token, req.Domain`. Update to use new signature (no domain param). The TCP server will need to handle `[]*ent.Tunnel` — it can find the specific tunnel by matching `req.Domain` against the returned list.

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/tunnel/...`

- [ ] **Step 5: Commit**

```bash
git add internal/tunnel/auth_helpers.go internal/tunnel/grpc_service_helpers.go internal/tunnel/server.go
git commit -m "feat: auth helpers return all user tunnels for multi-tunnel support"
```

---

### Task 9: Update gRPC Server for Multi-Domain Streams

**Files:**
- Modify: `internal/tunnel/grpc_service.go`

This is the most complex task.

- [ ] **Step 1: Add `Domains` to TunnelStream struct**

The `TunnelStream` struct at `grpc_service.go:89-119` already has `UserID`. Add `Domains`:

```go
type TunnelStream struct {
    // ... existing fields (Domain stays for backward compat logging) ...
    Domains    []string  // All domains served by this stream
    TunnelIDs  []uint32  // All tunnel IDs served by this stream
    // ... rest unchanged ...
}
```

Replace single `TunnelID uint32` with `TunnelIDs []uint32`.

- [ ] **Step 2: Add `userStreams` map to GRPCTunnelServer**

At `grpc_service.go:48-50`, add alongside `tunnelStreams`:

```go
// Active tunnel streams
tunnelStreams    map[string]*TunnelStream  // domain -> stream
tunnelStreamsMux sync.RWMutex
userStreams      map[uint32]*TunnelStream  // userID -> stream (shares tunnelStreamsMux)
```

Both maps share `tunnelStreamsMux` to ensure atomic updates.

Initialize `userStreams: make(map[uint32]*TunnelStream)` in constructor.

- [ ] **Step 3: Rewrite `EstablishTunnel` for multi-domain**

The current flow at lines 297-412 does:
1. Receive handshake → authenticate → get single tunnel
2. UpdateClientIP for one tunnel
3. Create TunnelStream with one domain
4. Register one domain in tunnelStreams
5. Defer: delete one domain, clean up one tunnel's Caddy route
6. Send response with single domain/port in TunnelStatus

New flow:
1. Receive handshake → authenticate → get `[]*ent.Tunnel` + `userID`
2. UpdateClientIP for ALL tunnels (iterate list)
3. Create TunnelStream with `Domains` list and `TunnelIDs` list
4. Register ALL domains in `tunnelStreams` map, register in `userStreams[userID]`
5. Defer: iterate `Domains`, delete each from `tunnelStreams`; delete from `userStreams`; call `UpdateClientIP(ctx, id, "")` for each tunnel
6. Send response with `Routes` list in TunnelStatus (populate from tunnels). Set `Domain` to first tunnel's domain for logging compat. Remove `chosenPort` logic (line 339-342).
7. The `TunnelStream.TargetPort` field becomes unused — remove or leave for logging.

- [ ] **Step 4: Update ControlChannel for token-based lookup**

At `grpc_service.go:416-490`, the current flow:
1. Receive `ControlHandshake` → extract `domain` (line 428-434)
2. Look up `tunnelStreams[domain]` (line 438-440)

New flow:
1. Receive `ControlHandshake` → extract `token` (new field)
2. Validate token → get `userID` (call `tokenRepo.GetByToken`)
3. Look up `userStreams[userID]` (using `tunnelStreamsMux`)
4. Bind control stream to that TunnelStream

- [ ] **Step 5: Add ConfigUpdate push method**

```go
func (s *GRPCTunnelServer) PushConfigUpdate(userID uint32, tunnels []*ent.Tunnel, reason proto.ConfigUpdateReason) error {
    s.tunnelStreamsMux.RLock()
    stream, exists := s.userStreams[userID]
    s.tunnelStreamsMux.RUnlock()
    if !exists {
        return nil
    }

    routes := make([]*proto.TunnelRouteConfig, len(tunnels))
    for i, t := range tunnels {
        routes[i] = &proto.TunnelRouteConfig{
            Domain:     t.Domain,
            TargetHost: t.TargetHost,
            TargetPort: int32(t.TargetPort),
            IsEnabled:  t.IsEnabled,
        }
    }

    stream.controlMux.RLock()
    ctrl := stream.ControlStream
    stream.controlMux.RUnlock()
    if ctrl == nil {
        return nil // No control channel, CLI will get state on reconnect
    }

    msg := &proto.ControlMessage{
        Timestamp: time.Now().Unix(),
        MessageType: &proto.ControlMessage_ConfigUpdate{
            ConfigUpdate: &proto.ConfigUpdate{
                Routes: routes,
                Reason: reason,
            },
        },
    }
    return ctrl.Send(msg)
}
```

- [ ] **Step 6: Add AddDomainToStream and RemoveDomainFromStream methods**

```go
func (s *GRPCTunnelServer) AddDomainToStream(userID uint32, domain string) {
    s.tunnelStreamsMux.Lock()
    defer s.tunnelStreamsMux.Unlock()
    if stream, exists := s.userStreams[userID]; exists {
        s.tunnelStreams[domain] = stream
        stream.Domains = append(stream.Domains, domain)
    }
}

func (s *GRPCTunnelServer) RemoveDomainFromStream(userID uint32, domain string) {
    s.tunnelStreamsMux.Lock()
    defer s.tunnelStreamsMux.Unlock()
    delete(s.tunnelStreams, domain)
    if stream, exists := s.userStreams[userID]; exists {
        for i, d := range stream.Domains {
            if d == domain {
                stream.Domains = append(stream.Domains[:i], stream.Domains[i+1:]...)
                break
            }
        }
    }
}
```

- [ ] **Step 7: Verify compilation**

Run: `go build ./internal/tunnel/...`

- [ ] **Step 8: Commit**

```bash
git add internal/tunnel/grpc_service.go internal/tunnel/grpc_service_helpers.go
git commit -m "feat: gRPC server multi-domain stream registration and config push"
```

---

### Task 10: Wire Config Push from Tunnel Service

**Files:**
- Create: `internal/interfaces/config_pusher.go`
- Modify: `internal/service/tunnel_service.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Create TunnelConfigPusher interface**

Create `internal/interfaces/config_pusher.go`:

```go
package interfaces

import "github.com/osa911/giraffecloud/internal/db/ent"

// TunnelConfigPusher pushes tunnel config updates to connected CLI clients.
// Implemented by GRPCTunnelServer to avoid circular imports.
type TunnelConfigPusher interface {
    PushConfigUpdate(userID uint32, tunnels []*ent.Tunnel, reason int32) error
    AddDomainToStream(userID uint32, domain string)
    RemoveDomainFromStream(userID uint32, domain string)
}
```

Note: Use `int32` for reason instead of `proto.ConfigUpdateReason` to avoid importing proto package in interfaces. The GRPCTunnelServer implementation will cast to proto type.

- [ ] **Step 2: Add configPusher to tunnel service**

Add field and setter to `tunnelService` struct in `tunnel_service.go`:

```go
type tunnelService struct {
    repo           repository.TunnelRepository
    caddyService   CaddyService
    config         *config.Config
    configPusher   interfaces.TunnelConfigPusher
}

func (s *tunnelService) SetConfigPusher(pusher interfaces.TunnelConfigPusher) {
    s.configPusher = pusher
}
```

- [ ] **Step 3: Push config after Create/Update/Delete**

At the end of `CreateTunnel`, after successful creation:
```go
if s.configPusher != nil {
    tunnels, _ := s.repo.GetByUserID(ctx, userID)
    s.configPusher.PushConfigUpdate(userID, tunnels, 1) // TUNNEL_CREATED
    s.configPusher.AddDomainToStream(userID, domain)
}
```

At the end of `UpdateTunnel`, after successful update:
```go
if s.configPusher != nil {
    tunnels, _ := s.repo.GetByUserID(ctx, userID)
    s.configPusher.PushConfigUpdate(userID, tunnels, 2) // TUNNEL_UPDATED
}
```

At the end of `DeleteTunnel`, after successful deletion:
```go
if s.configPusher != nil {
    tunnels, _ := s.repo.GetByUserID(ctx, tunnel.UserID)
    s.configPusher.PushConfigUpdate(tunnel.UserID, tunnels, 3) // TUNNEL_DELETED
    s.configPusher.RemoveDomainFromStream(tunnel.UserID, tunnel.Domain)
}
```

- [ ] **Step 4: Wire in server.go**

In `server.go` `Init()`, after creating the hybrid router, call:
```go
tunnelService.SetConfigPusher(hybridRouter.Grpc())
```

This requires the tunnel service to be a concrete type or the interface to expose `SetConfigPusher`. Adjust as needed based on current wiring pattern.

- [ ] **Step 5: Verify compilation**

Run: `go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/interfaces/config_pusher.go internal/service/tunnel_service.go internal/server/server.go
git commit -m "feat: wire config push from tunnel service to gRPC server"
```

---

## Chunk 3: CLI Changes

### Task 11: Add RouteTable to CLI

**Files:**
- Create: `internal/tunnel/route_table.go`

- [ ] **Step 1: Create RouteTable**

```go
package tunnel

import (
    "fmt"
    "sync"

    "github.com/osa911/giraffecloud/internal/logging"
    "github.com/osa911/giraffecloud/internal/tunnel/proto"
)

type Route struct {
    Domain     string
    TargetHost string
    TargetPort int32
}

func (r *Route) Target() string {
    return fmt.Sprintf("%s:%d", r.TargetHost, r.TargetPort)
}

type RouteTable struct {
    mu     sync.RWMutex
    routes map[string]*Route
    logger *logging.Logger
}

func NewRouteTable() *RouteTable {
    return &RouteTable{
        routes: make(map[string]*Route),
        logger: logging.GetGlobalLogger(),
    }
}

func (rt *RouteTable) Resolve(domain string) *Route {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    return rt.routes[domain]
}

func (rt *RouteTable) Update(configs []*proto.TunnelRouteConfig) {
    rt.mu.Lock()
    defer rt.mu.Unlock()

    newRoutes := make(map[string]*Route, len(configs))
    for _, cfg := range configs {
        if cfg.IsEnabled {
            newRoutes[cfg.Domain] = &Route{
                Domain:     cfg.Domain,
                TargetHost: cfg.TargetHost,
                TargetPort: cfg.TargetPort,
            }
        }
    }
    rt.routes = newRoutes

    rt.logger.Info("Route table updated: serving %d tunnels", len(rt.routes))
    for domain, route := range rt.routes {
        rt.logger.Info("  %s -> %s", domain, route.Target())
    }
}

func (rt *RouteTable) Count() int {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    return len(rt.routes)
}

func (rt *RouteTable) All() []*Route {
    rt.mu.RLock()
    defer rt.mu.RUnlock()
    routes := make([]*Route, 0, len(rt.routes))
    for _, r := range rt.routes {
        routes = append(routes, r)
    }
    return routes
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/tunnel/...`

- [ ] **Step 3: Commit**

```bash
git add internal/tunnel/route_table.go
git commit -m "feat: add RouteTable for multi-target routing"
```

---

### Task 12: Update gRPC Client for Multi-Tunnel

**Files:**
- Modify: `internal/tunnel/grpc_client.go`

- [ ] **Step 1: Replace domain/targetPort with RouteTable**

Current constructor at `grpc_client.go:125`:
```go
func NewGRPCTunnelClient(serverAddr, domain, token string, targetPort int32, config *GRPCClientConfig) *GRPCTunnelClient
```

Change to:
```go
func NewGRPCTunnelClient(serverAddr, token string, config *GRPCClientConfig) *GRPCTunnelClient
```

Remove `domain string` and `targetPort int32` fields from the struct. Add `routeTable *RouteTable`. Initialize in constructor: `routeTable: NewRouteTable()`.

- [ ] **Step 2: Update handshake to populate route table**

In the handshake response handler, after receiving `TunnelStatus`:

```go
if status.Routes != nil && len(status.Routes) > 0 {
    c.routeTable.Update(status.Routes)
}
```

- [ ] **Step 3: Update all local proxy dial calls**

Search for all `net.Dial` or `net.DialTimeout` calls that use `c.targetPort` or `c.domain` to determine the local target. Replace with route table lookup:

```go
route := c.routeTable.Resolve(domain)
if route == nil {
    return fmt.Errorf("no route for domain: %s", domain)
}
conn, err := net.DialTimeout("tcp", route.Target(), timeout)
```

The `domain` is extracted from the incoming `HTTPRequest.Host` header.

- [ ] **Step 4: Handle ConfigUpdate control messages**

In the control message handler (find the switch on control message type), add:

```go
case *proto.ControlMessage_ConfigUpdate:
    update := msg.GetConfigUpdate()
    c.routeTable.Update(update.Routes)
    c.logger.Info("Config update received: now serving %d tunnels", c.routeTable.Count())
```

- [ ] **Step 5: Update all callers of NewGRPCTunnelClient**

Run: `grep -rn "NewGRPCTunnelClient" internal/ cmd/ --include="*.go"`

Update each call site to remove `domain` and `targetPort` params.

- [ ] **Step 6: Verify compilation**

Run: `go build ./internal/tunnel/...`

- [ ] **Step 7: Commit**

```bash
git add internal/tunnel/grpc_client.go
git commit -m "feat: gRPC client uses route table for multi-target proxying"
```

---

### Task 13: Update Tunnel Client (tunnel.go)

**Files:**
- Modify: `internal/tunnel/tunnel.go`

- [ ] **Step 1: Update `Connect` signature**

Current at line 202:
```go
func (t *Tunnel) Connect(ctx context.Context, serverAddr, token, domain string, localPort int, tlsConfig *tls.Config) error
```

Change to:
```go
func (t *Tunnel) Connect(ctx context.Context, serverAddr, token string, tlsConfig *tls.Config) error
```

- [ ] **Step 2: Update internal proxy logic**

Find all places in `tunnel.go` where it dials `localhost:<port>` using the fixed local port and replace with route table lookup via the gRPC client's route table.

- [ ] **Step 3: Update TCP tunnel connection similarly**

The TCP tunnel (for WebSocket) also dials local targets. Find where it uses domain/port to connect locally and update to use the route table.

- [ ] **Step 4: Update all callers of Connect**

The main caller is in `cmd/giraffecloud/main.go:190`:
```go
err = t.Connect(ctx, serverAddr, cfg.Token, cfg.Domain, cfg.LocalPort, tlsConfig)
```

This will be updated in Task 14.

- [ ] **Step 5: Verify compilation**

Run: `go build ./internal/tunnel/...`

- [ ] **Step 6: Commit**

```bash
git add internal/tunnel/tunnel.go
git commit -m "feat: tunnel client uses server-provided routes"
```

---

### Task 14: Update CLI Connect Command

**Files:**
- Modify: `cmd/giraffecloud/main.go`

- [ ] **Step 1: Simplify connect command**

Update the `connectCmd.Run` at line 190:

```go
err = t.Connect(ctx, serverAddr, cfg.Token, tlsConfig)
```

Remove all domain-related logic:
- `domainFlag` extraction (line 104)
- `cfg.Domain = domainFlag` (line 113)
- Multiple tunnel error handling (lines 198-231)
- Domain saving to config (lines 238-244)

- [ ] **Step 2: Update connect command description**

```go
var connectCmd = &cobra.Command{
    Use:   "connect",
    Short: "Connect to GiraffeCloud and serve all configured tunnels",
    Long: `Connect to GiraffeCloud and serve all your configured tunnels.
The CLI will proxy requests for each domain to its configured target in your local network.
Configure tunnels at https://giraffecloud.xyz/dashboard/tunnels

Examples:
  giraffecloud connect    # Connect and serve all enabled tunnels`,
```

- [ ] **Step 3: Remove --domain flag registration**

In `init()` at line 519, remove:
```go
connectCmd.Flags().String("domain", "", "Domain of the tunnel to connect to (required if you have multiple tunnels)")
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./cmd/giraffecloud/...`

- [ ] **Step 5: Commit**

```bash
git add cmd/giraffecloud/main.go
git commit -m "feat: simplify CLI connect to serve all tunnels"
```

---

## Chunk 4: Frontend Changes

### Task 15: Update Frontend Tunnel Types

**Files:**
- Modify: `apps/web/src/types/tunnel.ts`

- [ ] **Step 1: Add `target_host` to all tunnel interfaces**

Add `target_host: string` after `domain` in:
- `Tunnel` interface (line 9)
- `TunnelCreateResponse` — if it has its own fields, add there too
- `TunnelFormData` (line 30) — add `target_host: string`

Remove `token` from `TunnelCreateResponse` if present (per-tunnel tokens are removed).

- [ ] **Step 2: Commit**

```bash
git add apps/web/src/types/tunnel.ts
git commit -m "feat: add target_host to frontend tunnel types"
```

---

### Task 16: Update Tunnel Dialog

**Files:**
- Modify: `apps/web/src/components/dashboard/tunnels/TunnelDialog.tsx`

- [ ] **Step 1: Add `target_host` to form schema**

In the Zod schema (around line 48-57), add:

```typescript
target_host: z.string().default("localhost"),
```

- [ ] **Step 2: Add default value**

In form `defaultValues`, add:
```typescript
target_host: tunnel?.target_host || "localhost",
```

- [ ] **Step 3: Add Target Host input field**

After the domain section and before the port input, add a FormField for `target_host`:

```tsx
<FormField
    control={form.control}
    name="target_host"
    render={({ field }) => (
        <FormItem>
            <FormLabel>Target Host</FormLabel>
            <FormControl>
                <Input placeholder="localhost" {...field} />
            </FormControl>
            <FormDescription>
                IP or hostname in your local network (e.g., 192.168.1.5, my-nas.local)
            </FormDescription>
            <FormMessage />
        </FormItem>
    )}
/>
```

- [ ] **Step 4: Ensure onSubmit includes target_host**

Check the `onSubmit` function sends `target_host` in the API payload.

- [ ] **Step 5: Commit**

```bash
git add apps/web/src/components/dashboard/tunnels/TunnelDialog.tsx
git commit -m "feat: add target host input to tunnel dialog"
```

---

### Task 17: Update Tunnel List

**Files:**
- Modify: `apps/web/src/components/dashboard/tunnels/TunnelList.tsx`

- [ ] **Step 1: Update table header**

Change "Port" to "Target" in the table header.

- [ ] **Step 2: Update table row**

Replace port-only display with:

```tsx
<TableCell>
    {tunnel.target_host === "localhost"
        ? `:${tunnel.target_port}`
        : `${tunnel.target_host}:${tunnel.target_port}`}
</TableCell>
```

- [ ] **Step 3: Commit**

```bash
git add apps/web/src/components/dashboard/tunnels/TunnelList.tsx
git commit -m "feat: show full target in tunnel list"
```

---

### Task 18: Update Getting Started Page

**Files:**
- Modify: `apps/web/src/components/dashboard/GettingStartedPage.tsx`

- [ ] **Step 1: Update connect command examples**

Replace `giraffecloud connect --domain YOUR_DOMAIN` with `giraffecloud connect`.

- [ ] **Step 2: Add LAN routing section**

Add after existing steps:

```tsx
<div className="space-y-2">
    <h4 className="font-medium">Exposing LAN Services</h4>
    <p className="text-sm text-muted-foreground">
        You can route different domains to different machines in your local network.
        Configure each tunnel&apos;s target host in the dashboard — for example,
        point one domain to 192.168.1.5:8080 and another to 192.168.1.10:3000.
        One CLI instance serves them all.
    </p>
</div>
```

- [ ] **Step 3: Remove all --domain references**

Search for `--domain` in the file and remove or update.

- [ ] **Step 4: Commit**

```bash
git add apps/web/src/components/dashboard/GettingStartedPage.tsx
git commit -m "feat: update getting started for multi-tunnel connect"
```

---

## Chunk 5: Documentation & Final Verification

### Task 19: Update Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/installation.md`
- Modify: `docs/hybrid-tunnel-architecture.md`
- Modify: `llms.txt`

- [ ] **Step 1: Update README.md** — Add LAN routing to features, update quick start.
- [ ] **Step 2: Update installation.md** — Update connect flow.
- [ ] **Step 3: Update hybrid-tunnel-architecture.md** — Add multi-target routing section.
- [ ] **Step 4: Update llms.txt** — Mention LAN routing capability.
- [ ] **Step 5: Commit**

```bash
git add README.md docs/ llms.txt
git commit -m "docs: update documentation for multi-target LAN routing"
```

---

### Task 20: Full Build & Smoke Test

- [ ] **Step 1: Full backend build**

Run: `go build ./...`
Expected: Clean compilation.

- [ ] **Step 2: Regenerate Ent and Proto**

Run: `go generate ./internal/db/ent && make proto`
Expected: All generated code is up to date.

- [ ] **Step 3: Frontend build**

Run: `cd apps/web && npm run build`
Expected: Clean build.

- [ ] **Step 4: Final commit if any generated files changed**

```bash
git add -A
git commit -m "chore: regenerate ent and proto code"
```
