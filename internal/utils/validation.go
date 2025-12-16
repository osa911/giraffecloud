package utils

import (
	"regexp"
)

// DomainRegex is the regex for validating domains
// It allows for subdomains and requires at least one dot (e.g. example.com)
// It does not allow for IP addresses or localhost
var DomainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// IsValidDomain checks if the provided string is a valid domain name
func IsValidDomain(domain string) bool {
	if len(domain) > 253 {
		return false
	}
	return DomainRegex.MatchString(domain)
}
