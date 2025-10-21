#!/bin/bash

# Generate TLS certificates for local development
CERT_DIR="certs"

# Create directory for certificates
mkdir -p "$CERT_DIR"

# Create config file for certificate generation
cat > "$CERT_DIR/openssl.conf" << EOF
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
O = GiraffeCloud-Local
CN = localhost

[req_ext]
subjectAltName = @alt_names

[v3_ext]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth,clientAuth

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
IP.1 = 127.0.0.1
EOF

echo "Generating CA certificate..."
# Generate CA private key and certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt" \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud-Local/CN=GiraffeCloud Local CA" \
  -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign"

echo "Generating server certificate..."
# Generate server private key
openssl genrsa -out "$CERT_DIR/tunnel.key" 4096

# Generate server CSR with config file
openssl req -new -key "$CERT_DIR/tunnel.key" \
  -out "$CERT_DIR/tunnel.csr" \
  -config "$CERT_DIR/openssl.conf"

# Generate server certificate
openssl x509 -req -in "$CERT_DIR/tunnel.csr" \
  -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
  -out "$CERT_DIR/tunnel.crt" -days 365 \
  -extfile "$CERT_DIR/openssl.conf" -extensions v3_ext

# Set proper permissions
chmod 600 "$CERT_DIR/ca.key" "$CERT_DIR/tunnel.key"
chmod 644 "$CERT_DIR/tunnel.crt" "$CERT_DIR/ca.crt"

# Clean up
rm "$CERT_DIR/tunnel.csr" "$CERT_DIR/ca.srl" "$CERT_DIR/openssl.conf"

echo "✅ Local certificates generated successfully in $CERT_DIR/"
echo "   - CA Certificate: $CERT_DIR/ca.crt"
echo "   - Server Certificate: $CERT_DIR/tunnel.crt"
echo "   - Server Key: $CERT_DIR/tunnel.key"



