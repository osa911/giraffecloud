#!/bin/sh
set -e

# Generate the GiraffeCloud tunnel CA (long-lived).
# Re-running overwrites the existing CA — clients will need to re-fetch ca.crt
# and re-login to receive a client cert signed by the new CA. Callers (e.g.
# ensure-tunnel-certs.sh) decide when this is safe.

CERT_DIR="${CERT_DIR:-/app/certs}"
CA_CRT="$CERT_DIR/ca.crt"
CA_KEY="$CERT_DIR/ca.key"
CA_DAYS="${CA_DAYS:-3650}"

mkdir -p "$CERT_DIR"

echo "Generating tunnel CA ($CA_DAYS days)..."
openssl req -x509 -newkey rsa:4096 -days "$CA_DAYS" -nodes \
  -keyout "$CA_KEY" -out "$CA_CRT" \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud/CN=GiraffeCloud Tunnel CA" \
  -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign"

chmod 600 "$CA_KEY"
chmod 644 "$CA_CRT"

# Reset the serial counter so the next leaf doesn't collide with an older one
rm -f "$CERT_DIR/ca.srl"

echo "Tunnel CA written to $CA_CRT (valid $CA_DAYS days)"
