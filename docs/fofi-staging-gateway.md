# Operate the Fofi staging gateway

The shared Caddy process loads the Fofi client certificate when it loads this configuration. Missing, unreadable, invalid, or mismatched files can prevent the whole proxy from starting. Preflight is an operator step and does not run automatically on every restart.

## Before a change

1. Save the running configuration in a root-only directory outside Git. Create a mode 0700 directory at `/root/fofi-proxy-recovery` and set `umask 077`. Fetch `http://172.20.0.4:2019/config/` into a new temporary file with `curl --fail --silent --show-error --noproxy '*'`. Check the exit status and validate the temporary file with `sudo scripts/preflight-fofi-gateway.sh TEMPORARY_FILE` before replacing the previous snapshot at `/root/fofi-proxy-recovery/live-caddy.json`. Keep the previous snapshot if the fetch or validation fails. Save the matching Fofi client certificate, private key, and CA certificate there too. The live configuration can contain customer routes and secrets. It captures only routes present at that instant. Take a fresh snapshot before a planned change and prevent concurrent route updates during recovery.
2. Provision `/data/fofi-staging-mtls/ca.crt`, `client.crt`, and `client.key` in the existing `giraffecloud_caddy_data` volume. On this server the host directory is `/var/lib/docker/volumes/giraffecloud_caddy_data/_data/fofi-staging-mtls`. Make the directory root-owned and mode 0700, and the private key root-owned and mode 0600. The Caddy container currently runs as root. Do not copy the CA private key into this volume.
3. Run `sudo scripts/preflight-fofi-gateway.sh internal/caddy/caddy.config.json` from the repository root. It checks the certificate chain, client purpose, expiry, key match, and the complete Caddy configuration in an isolated container. It makes no changes to the running proxy.
4. Verify the new gateway separately with its CA and client credentials, then load the candidate configuration using the existing Caddy admin workflow. Confirm both Fofi and GiraffeCloud HTTPS endpoints. Do not restart the shared proxy if preflight fails.

The script uses the `giraffecloud_caddy` image and the `giraffecloud_caddy_data` volume. Set `CADDY_IMAGE` only when testing the exact image intended for deployment. Supply the certificate directory as the second argument when checking a staged set of files.

## Recover or rotate certificates

If Caddy is running and a candidate fails validation, leave the running configuration in place. Repair or restore the certificate files, repeat preflight, and verify the gateway before loading a replacement.

If Caddy is down, recovery causes an outage until it starts again. The container starts with `/etc/caddy/caddy.json`, currently bind-mounted from `/root/giraffecloud/internal/caddy/caddy.config.json`. Confirm the source with `docker inspect giraffecloud_caddy_1` before following these steps. The repository's static configuration may lack dynamically added customer routes.

1. Pause changes to customer routes. Restore the matching certificate files to their original paths, with the permissions above. Keep the protected snapshot and the previous startup file until recovery is verified.
2. Validate the snapshot: `sudo scripts/preflight-fofi-gateway.sh /root/fofi-proxy-recovery/live-caddy.json`. Stop if validation fails. Verify that the snapshot is recent enough to contain the routes you need; it cannot recover routes added after the snapshot.
3. Confirm that the container is stopped: `docker inspect -f '{{.State.Running}}' giraffecloud_caddy_1` must print `false`. If Docker is automatically restarting it, stop the container first. Save a protected copy of the existing startup file, and set the existing startup file to root-owned mode 0600 before copying because the snapshot can contain secrets. Copy the validated snapshot **into the existing mounted file**, preserving its inode: `cat /root/fofi-proxy-recovery/live-caddy.json > /root/giraffecloud/internal/caddy/caddy.config.json`. Do not use a rename or atomic replacement: an existing file bind mount can continue to point to the old inode. This emergency step modifies a tracked file in the deployment checkout; preserve the old copy and reconcile it later without discarding the recovered routes.
4. Confirm the file matches the snapshot with `cmp -s /root/fofi-proxy-recovery/live-caddy.json /root/giraffecloud/internal/caddy/caddy.config.json`. Only if it matches, run `docker start giraffecloud_caddy_1`. Check the container status, GiraffeCloud HTTPS, Fofi HTTPS, and representative customer routes. If the copy was interrupted or does not match, repeat it before starting the container.

A snapshot is not an automatic recovery system. An unattended restart can still fail if certificate files disappear, expire, or become invalid. Preflight and credential monitoring must be run separately.

For rotation, create a new client certificate signed by the gateway's trusted CA. Stage the new certificate and key in a separate root-only directory, check them with the second preflight argument, and test the gateway connection. Keep the previous matching files until the new files and configuration have been validated and both public endpoints respond. The gateway CA, server certificate, and their rotation are managed with the Fofi staging deployment. Coordinate CA changes on both ends before removing the old trust.

The current Fofi certificate set expires in February 2029. No automatic expiry alert or certificate rotation is configured.

## Provision Fofi container keys

The Fofi API runs as UID and GID 1000. The staged gateway image runs as UID and GID 65532. On this host, keep both parent directories owned by root with mode 0700. After placing each private key in its intended file, apply:

```sh
chown root:root /etc/fofi-staging/secrets /etc/fofi-staging/gateway-mtls
chmod 0700 /etc/fofi-staging/secrets /etc/fofi-staging/gateway-mtls
chown root:1000 /etc/fofi-staging/secrets/apple-sign-in.p8
chmod 0640 /etc/fofi-staging/secrets/apple-sign-in.p8
chown root:65532 /etc/fofi-staging/gateway-mtls/server.key
chmod 0640 /etc/fofi-staging/gateway-mtls/server.key
```

Docker bind-mounts the files read-only into the respective containers; the restricted host directories are not mounted into them. These numeric groups are tied to the current images. Confirm the image identity before changing image versions, and account for Docker user-namespace remapping if it is enabled. Do not print private keys.

Check that the API can read its mounted key with `docker exec --user 1000:1000 fofi-staging_api_1 sh -c 'test -r /run/fofi/apple-sign-in.p8'`. Check the staged gateway key without starting a server:

```sh
docker run --rm --network none --user 65532:65532 --entrypoint /bin/sh \
  -v /etc/fofi-staging/gateway-mtls/server.key:/run/fofi/gateway.key:ro \
  fofi-staging-edge:mtls-c2c1db5d6913 -c 'test -r /run/fofi/gateway.key'
```

Both commands must exit successfully. Repeat each test with `--user 65534:65534` and confirm that it fails. This tests file access without displaying the keys.
