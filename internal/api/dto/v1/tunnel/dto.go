package tunnel

import (
	"time"
)

// CreateTunnelRequest represents the payload for creating a new tunnel
type CreateTunnelRequest struct {
	Name       string `json:"name" binding:"required"`
	Protocol   string `json:"protocol" binding:"required,oneof=http https tcp udp"`
	LocalPort  int    `json:"localPort" binding:"required,min=1,max=65535"`
	RemoteHost string `json:"remoteHost" binding:"required"`
}

// UpdateTunnelRequest represents the payload for updating an existing tunnel
type UpdateTunnelRequest struct {
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	LocalPort  int    `json:"localPort"`
	RemoteHost string `json:"remoteHost"`
	IsEnabled  bool   `json:"isEnabled"`
}

// TunnelResponse represents a tunnel in API responses
type TunnelResponse struct {
	ID         uint      `json:"id"`
	Name       string    `json:"name"`
	Protocol   string    `json:"protocol"`
	LocalPort  int       `json:"localPort"`
	RemoteHost string    `json:"remoteHost"`
	Status     string    `json:"status"`
	IsEnabled  bool      `json:"isEnabled"`
	UserID     uint      `json:"userId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// TunnelActionRequest represents a request to start or stop a tunnel
type TunnelActionRequest struct {
	Action string `json:"action" binding:"required,oneof=start stop"`
}

// TunnelActionResponse represents the response to a tunnel action request
type TunnelActionResponse struct {
	ID     uint   `json:"id"`
	Status string `json:"status"`
}

// ListTunnelsResponse represents a paginated list of tunnels
type ListTunnelsResponse struct {
	Tunnels    []TunnelResponse `json:"tunnels"`
	TotalCount int64            `json:"totalCount"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
}