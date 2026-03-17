package interfaces

import "github.com/osa911/giraffecloud/internal/db/ent"

// TunnelConfigPusher pushes config updates to connected CLI clients
type TunnelConfigPusher interface {
	PushConfigUpdate(userID uint32, tunnels []*ent.Tunnel, reason int32) error
	AddDomainToStream(userID uint32, domain string)
	RemoveDomainFromStream(userID uint32, domain string)
}
