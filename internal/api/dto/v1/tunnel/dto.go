package tunnel

import "time"

// CreateRequest represents the request for creating a tunnel
type CreateRequest struct {
	Domain     string `json:"domain"` // Optional: auto-generated if empty
	TargetPort int    `json:"target_port" binding:"required,min=1,max=65535"`
}

// CreateResponse includes the tunnel token (only returned on creation)
type CreateResponse struct {
	ID         int       `json:"id"`
	Domain     string    `json:"domain"`
	Token      string    `json:"token"` // ⚠️ Only included on creation
	TargetPort int       `json:"target_port"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Response represents a tunnel without the sensitive token (for list/get/update)
type Response struct {
	ID         int       `json:"id"`
	Domain     string    `json:"domain"`
	TargetPort int       `json:"target_port"`
	IsActive   bool      `json:"is_active"`
	ClientIP   string    `json:"client_ip,omitempty"` // Empty if not connected
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UpdateRequest represents the request for updating a tunnel
type UpdateRequest struct {
	IsActive   *bool `json:"is_active,omitempty"`
	TargetPort *int  `json:"target_port,omitempty" binding:"omitempty,min=1,max=65535"`
}

// FreeSubdomainResponse represents the available free subdomain for a user
type FreeSubdomainResponse struct {
	Domain    string `json:"domain"`
	Available bool   `json:"available"`
}
