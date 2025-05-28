package tunnel

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// L7Config holds Layer 7 processing configuration
type L7Config struct {
	EnableHTTP       bool     // Enable HTTP processing
	AllowedMethods   []string // Allowed HTTP methods
	MaxHeaderSize    int      // Maximum header size in bytes
	MaxURILength     int      // Maximum URI length
	StripPath        string   // Path to strip from requests
	AddResponseHeaders map[string]string // Headers to add to responses
}

// DefaultL7Config returns default Layer 7 settings
func DefaultL7Config() *L7Config {
	return &L7Config{
		EnableHTTP: true,
		AllowedMethods: []string{
			"GET", "POST", "PUT", "DELETE",
			"HEAD", "OPTIONS", "PATCH",
		},
		MaxHeaderSize: 32 * 1024, // 32KB
		MaxURILength:  8 * 1024,  // 8KB
		AddResponseHeaders: map[string]string{
			"X-Tunnel-ID": "", // Will be set per tunnel
		},
	}
}

// L7Handler handles Layer 7 (HTTP) processing
type L7Handler struct {
	config *L7Config
	mu     sync.RWMutex

	// Statistics
	stats struct {
		totalRequests     uint64
		httpErrors       uint64
		methodDenied     uint64
		uriTooLong       uint64
		headersTooLarge  uint64
		invalidProtocol  uint64
	}
}

// NewL7Handler creates a new Layer 7 handler
func NewL7Handler(config *L7Config) *L7Handler {
	if config == nil {
		config = DefaultL7Config()
	}
	return &L7Handler{config: config}
}

// ProcessRequest processes an HTTP request
func (h *L7Handler) ProcessRequest(reader *bufio.Reader) (*http.Request, error) {
	// Read the first line to determine if it's HTTP
	firstLine, err := reader.Peek(5)
	if err != nil {
		return nil, fmt.Errorf("error peeking request: %w", err)
	}

	// Check if it looks like HTTP
	if !isHTTPMethod(string(firstLine)) {
		h.stats.invalidProtocol++
		return nil, fmt.Errorf("not an HTTP request")
	}

	// Read the full request
	req, err := http.ReadRequest(reader)
	if err != nil {
		h.stats.httpErrors++
		return nil, fmt.Errorf("error reading HTTP request: %w", err)
	}

	// Validate method
	if !h.isMethodAllowed(req.Method) {
		h.stats.methodDenied++
		return nil, fmt.Errorf("method not allowed: %s", req.Method)
	}

	// Check URI length
	if len(req.RequestURI) > h.config.MaxURILength {
		h.stats.uriTooLong++
		return nil, fmt.Errorf("URI too long")
	}

	// Strip path if configured
	if h.config.StripPath != "" && strings.HasPrefix(req.URL.Path, h.config.StripPath) {
		req.URL.Path = strings.TrimPrefix(req.URL.Path, h.config.StripPath)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}

	h.stats.totalRequests++
	return req, nil
}

// ProcessResponse processes an HTTP response
func (h *L7Handler) ProcessResponse(resp *http.Response, tunnelID string) error {
	// Add configured headers
	for k, v := range h.config.AddResponseHeaders {
		if k == "X-Tunnel-ID" {
			resp.Header.Set(k, tunnelID)
		} else {
			resp.Header.Set(k, v)
		}
	}
	return nil
}

// isMethodAllowed checks if the HTTP method is allowed
func (h *L7Handler) isMethodAllowed(method string) bool {
	for _, m := range h.config.AllowedMethods {
		if m == method {
			return true
		}
	}
	return false
}

// isHTTPMethod checks if the start of the request looks like an HTTP method
func isHTTPMethod(start string) bool {
	methods := []string{"GET", "POST", "PUT", "HEAD", "DELETE", "OPTIONS", "PATCH"}
	for _, method := range methods {
		if strings.HasPrefix(strings.ToUpper(start), method) {
			return true
		}
	}
	return false
}

// GetStats returns Layer 7 processing statistics
func (h *L7Handler) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"total_requests":      h.stats.totalRequests,
		"http_errors":        h.stats.httpErrors,
		"method_denied":      h.stats.methodDenied,
		"uri_too_long":       h.stats.uriTooLong,
		"headers_too_large":  h.stats.headersTooLarge,
		"invalid_protocol":   h.stats.invalidProtocol,
	}
}

// ACLRule represents a Layer 7 access control rule
type ACLRule struct {
	Type      string   // path, header, method, etc.
	Pattern   string   // regex pattern to match
	Values    []string // allowed values
	Action    string   // allow, deny, redirect
	RedirectTo string  // URL for redirect action
}

// ACLMatcher handles Layer 7 access control
type ACLMatcher struct {
	rules []ACLRule
	mu    sync.RWMutex
}

// NewACLMatcher creates a new ACL matcher
func NewACLMatcher() *ACLMatcher {
	return &ACLMatcher{
		rules: make([]ACLRule, 0),
	}
}

// AddRule adds a new ACL rule
func (m *ACLMatcher) AddRule(rule ACLRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// MatchRequest checks if a request matches any ACL rules
func (m *ACLMatcher) MatchRequest(req *http.Request) (string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rule := range m.rules {
		switch rule.Type {
		case "path":
			if strings.HasPrefix(req.URL.Path, rule.Pattern) {
				return rule.Action, rule.RedirectTo
			}
		case "method":
			for _, method := range rule.Values {
				if req.Method == method {
					return rule.Action, rule.RedirectTo
				}
			}
		case "header":
			if value := req.Header.Get(rule.Pattern); value != "" {
				for _, allowed := range rule.Values {
					if value == allowed {
						return rule.Action, rule.RedirectTo
					}
				}
			}
		}
	}

	return "allow", ""
}