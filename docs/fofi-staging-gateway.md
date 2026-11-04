# Operate the Fofi staging gateway

The shared Caddy process loads the Fofi client certificate when it loads this configuration. Missing, unreadable, invalid, or mismatched files can prevent the whole proxy from starting. The preflight below is an operator step; it does not run automatically on every restart.

## Before a change

1. Keep a protected copy of the current live Caddy configuration and the matching Fofi client certificate, private key, and CA certificate. The live configuration can contain other customers' routes. Store the copy with mode 0600 in a root-only directory, outside Git, and preserve its original file references.
2. Provision `/data/fofi-staging-mtls/ca.crt`, `client.crt`, and `client.key` in the existing `giraffecloud_caddy_data` volume. On this server the corresponding host directory is `/var/lib/docker/volumes/giraffecloud_caddy_data/_data/fofi-staging-mtls`. Make the directory root-owned and mode 0700, and the private key root-owned and mode 0600. The Caddy container currently runs as root. Do not copy the CA private key into this volume.
3. Run `sudo scripts/preflight-fofi-gateway.sh internal/caddy/caddy.config.json` from the repository root. It checks the certificate chain, client purpose, expiry, key match, and the complete Caddy configuration in an isolated container. It makes no changes to the running proxy.
4. Verify the new gateway separately with its CA and client credentials, then load the candidate configuration using the existing Caddy admin workflow. Confirm both Fofi and GiraffeCloud HTTPS endpoints. Do not restart the shared proxy if preflight fails.

The script uses the `giraffecloud_caddy` image and the `giraffecloud_caddy_data` volume. Set `CADDY_IMAGE` only when testing the exact image intended for deployment. Supply the certificate directory as the second argument when checking a staged set of files.

## Recover or rotate certificates

If Caddy is running and a candidate fails validation, leave the running configuration in place. Repair or restore the certificate files, repeat preflight, and verify the gateway before loading a replacement. If Caddy is down, restore the protected matching certificate files at their original paths and validate the saved configuration before starting Caddy. This preserves the other routes in the saved configuration. Do not replace the live configuration with the repository's static configuration: the live proxy can contain additional routes.

For rotation, create a new client certificate signed by the gateway's trusted CA. Stage the new certificate and key in a separate root-only directory, check them with the second preflight argument, and test the gateway connection. Keep the previous matching files until the new files and configuration have been validated and both public endpoints respond. The gateway CA, server certificate, and their rotation are managed with the Fofi staging deployment. Coordinate CA changes on both ends before removing the old trust.

The current Fofi certificate set expires in February 2029. No automatic expiry alert or certificate rotation is configured.
