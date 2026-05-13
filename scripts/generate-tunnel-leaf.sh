#!/bin/sh
set -e

# Generate a tunnel server leaf certificate signed by the existing CA.
# Re-running rotates the server cert; clients trusting the same CA keep working
# without re-login. The server reloads from disk on every TLS handshake
# (see internal/tunnel/tls_config.go), so rotation is picked up live.

CERT_DIR="${CERT_DIR:-/app/certs}"
CA_CRT="$CERT_DIR/ca.crt"
CA_KEY="$CERT_DIR/ca.key"
LEAF_CRT="$CERT_DIR/tunnel.crt"
LEAF_KEY="$CERT_DIR/tunnel.key"
LEAF_DAYS="${LEAF_DAYS:-365}"
LEAF_CN="${LEAF_CN:-tunnel.giraffecloud.xyz}"

if [ ! -f "$CA_CRT" ] || [ ! -f "$CA_KEY" ]; then
  echo "ERROR: CA not found at $CA_CRT / $CA_KEY" >&2
  echo "Run generate-tunnel-ca.sh first." >&2
  exit 1
fi

mkdir -p "$CERT_DIR"

OPENSSL_CONF="$(mktemp)"
LEAF_CSR="$(mktemp)"
trap 'rm -f "$OPENSSL_CONF" "$LEAF_CSR" "$CERT_DIR/ca.srl"' EXIT

cat > "$OPENSSL_CONF" << EOF
[req]
default_bits = 4096
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn
x509_extensions = v3_ext

[dn]
C = US
ST = State
L = City
O = GiraffeCloud
CN = $LEAF_CN

[req_ext]
subjectAltName = @alt_names

[v3_ext]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth,clientAuth

[alt_names]
DNS.1 = $LEAF_CN
DNS.2 = *.$LEAF_CN
EOF

echo "Generating tunnel leaf ($LEAF_DAYS days, CN=$LEAF_CN)..."
openssl genrsa -out "$LEAF_KEY" 4096
openssl req -new -key "$LEAF_KEY" -out "$LEAF_CSR" -config "$OPENSSL_CONF"
openssl x509 -req -in "$LEAF_CSR" \
  -CA "$CA_CRT" -CAkey "$CA_KEY" -CAcreateserial \
  -out "$LEAF_CRT" -days "$LEAF_DAYS" \
  -extfile "$OPENSSL_CONF" -extensions v3_ext

chmod 600 "$LEAF_KEY"
chmod 644 "$LEAF_CRT"

echo "Tunnel leaf written to $LEAF_CRT (valid $LEAF_DAYS days)"
