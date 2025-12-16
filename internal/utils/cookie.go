package utils

import (
	"net/url"
	"os"
	"strings"

	"github.com/osa911/giraffecloud/internal/logging"
)

// CheckIfIP checks if a string is an IP address
func IsIPAddress(host string) bool {
	var logger = logging.GetGlobalLogger()
	logger.Info("isIPAddress_host: %s", host)
	// Simple check for IPv4 - looks for 4 segments of numbers separated by dots
	ipv4Parts := strings.Split(host, ".")
	if len(ipv4Parts) == 4 {
		for _, part := range ipv4Parts {
			// Check if each part contains only digits
			if !ContainsOnlyDigits(part) {
				return false
			}
		}
		return true
	}
	// Check for presence of colons which suggests IPv6
	return strings.Contains(host, ":")
}

// Helper function to check if a string contains only digits
func ContainsOnlyDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Get the appropriate cookie domain based on environment
func GetCookieDomain() string {
	env := os.Getenv("ENV")
	clientURL := os.Getenv("CLIENT_URL")

	var logger = logging.GetGlobalLogger()
	logger.Info("getCookieDomain_env: %s", env)
	logger.Info("getCookieDomain_clientURL: %s", clientURL)

	if env == "production" && clientURL != "" {
		parsableURL := clientURL
		if !strings.HasPrefix(parsableURL, "http://") && !strings.HasPrefix(parsableURL, "https://") {
			parsableURL = "https://" + parsableURL
		}

		parsedURL, err := url.Parse(parsableURL)
		logger.Info("getCookieDomain_parsedURL: %v", parsedURL)
		logger.Info("getCookieDomain_err: %v", err)
		if err != nil {
			return ""
		}

		host := parsedURL.Hostname()
		logger.Info("getCookieDomain_host: %s", host)

		if host == "" {
			return clientURL
		}

		logger.Info("getCookieDomain_host2: %s", host)
		if host == "localhost" || host == "127.0.0.1" || IsIPAddress(host) {
			return ""
		}

		parts := strings.Split(host, ".")
		logger.Info("getCookieDomain_parts: %v", parts)

		// Don't set cookies for tunnel subdomains
		if len(parts) >= 3 && parts[0] == "tunnel" {
			logger.Warn("Attempted to set cookie for tunnel subdomain, ignoring")
			return ""
		}

		// Always use root domain with leading dot, regardless of www or other subdomains
		if len(parts) >= 2 {
			// Find the root domain by checking from the end
			domainParts := parts
			// If we have www subdomain, remove it
			if len(parts) >= 3 && parts[0] == "www" {
				domainParts = parts[1:]
			}
			// Get the root domain (e.g., "giraffecloud.xyz")
			domain := domainParts[len(domainParts)-2] + "." + domainParts[len(domainParts)-1]
			logger.Info("getCookieDomain_domain: %s", domain)
			// Add leading dot to allow sharing between all subdomains
			return "." + domain
		}

		// Fallback to exact host if domain parsing fails
		logger.Info("getCookieDomain_fallback: %s", host)
		return host
	}

	return "" // Default empty string for development
}
