#!/usr/bin/env bash
set -euo pipefail

# Run before loading a Caddy configuration that uses the Fofi client certificate.
config=${1:?Usage: preflight-fofi-gateway.sh CADDY_CONFIG [CERT_DIR]}
cert_dir=${2:-/var/lib/docker/volumes/giraffecloud_caddy_data/_data/fofi-staging-mtls}
image=${CADDY_IMAGE:-giraffecloud_caddy}

for file in ca.crt client.crt client.key; do
    if [[ ! -r "$cert_dir/$file" ]]; then
        echo "Cannot read required certificate file: $cert_dir/$file" >&2
        exit 1
    fi
done
[[ -r "$config" ]] || { echo "Cannot read Caddy configuration: $config" >&2; exit 1; }
openssl verify -purpose sslclient -CAfile "$cert_dir/ca.crt" "$cert_dir/client.crt" >/dev/null
openssl x509 -in "$cert_dir/client.crt" -checkend 2592000 -noout >/dev/null || {
    echo 'Client certificate expires within 30 days.' >&2
    exit 1
}
cert_pub=$(openssl x509 -in "$cert_dir/client.crt" -pubkey -noout | openssl pkey -pubin -outform DER | openssl dgst -sha256)
key_pub=$(openssl pkey -in "$cert_dir/client.key" -pubout -outform DER | openssl dgst -sha256)
[[ "$cert_pub" == "$key_pub" ]] || { echo 'Client certificate and key do not match.' >&2; exit 1; }
cert_dir=$(cd "$cert_dir" && pwd -P)
config=$(cd "$(dirname "$config")" && printf '%s/%s' "$PWD" "$(basename "$config")")
docker run --rm --network none --entrypoint caddy \
    -v "$config:/tmp/caddy.json:ro" -v giraffecloud_caddy_data:/data:ro \
    -v "$cert_dir:/data/fofi-staging-mtls:ro" \
    "$image" validate --config /tmp/caddy.json
printf '%s\n' 'Fofi certificate and Caddy configuration preflight passed.'
