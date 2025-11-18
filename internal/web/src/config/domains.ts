/**
 * Reserved system domains configuration
 * These domains cannot be used for user tunnels
 */

// Get base domain from environment or use default
const BASE_DOMAIN = process.env.NEXT_PUBLIC_BASE_DOMAIN || "giraffecloud.xyz";

/**
 * Reserved system domains that cannot be used for tunnels
 */
export const RESERVED_DOMAINS = [
  BASE_DOMAIN, // giraffecloud.xyz
  `www.${BASE_DOMAIN}`, // www.giraffecloud.xyz
  `api.${BASE_DOMAIN}`, // api.giraffecloud.xyz
  `tunnel.${BASE_DOMAIN}`, // tunnel.giraffecloud.xyz
];

/**
 * Check if a domain is reserved for system use
 */
export function isReservedDomain(domain: string): boolean {
  return RESERVED_DOMAINS.includes(domain.toLowerCase());
}

/**
 * Get user-friendly error message for reserved domain
 */
export function getReservedDomainError(domain: string): string {
  return `'${domain}' is reserved for system use and cannot be used for tunnels`;
}
