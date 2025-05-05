#!/bin/sh

# Create directory for certificates
mkdir -p /app/certs

# Create config file for certificate generation
cat > /app/certs/openssl.conf << EOF
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
CN = tunnel.giraffecloud.xyz

[req_ext]
subjectAltName = @alt_names

[v3_ext]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature,keyEncipherment
extendedKeyUsage = serverAuth,clientAuth

[alt_names]
DNS.1 = tunnel.giraffecloud.xyz
DNS.2 = *.tunnel.giraffecloud.xyz
EOF

# Generate CA private key and certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout /app/certs/ca.key -out /app/certs/ca.crt \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud/CN=GiraffeCloud Tunnel CA" \
  -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign"

# Generate server private key
openssl genrsa -out /app/certs/tunnel.key 4096

# Generate server CSR with config file
openssl req -new -key /app/certs/tunnel.key \
  -out /app/certs/tunnel.csr \
  -config /app/certs/openssl.conf

# Generate server certificate
openssl x509 -req -in /app/certs/tunnel.csr \
  -CA /app/certs/ca.crt -CAkey /app/certs/ca.key -CAcreateserial \
  -out /app/certs/tunnel.crt -days 365 \
  -extfile /app/certs/openssl.conf -extensions v3_ext

# Set proper permissions
chmod 600 /app/certs/ca.key /app/certs/tunnel.key
chmod 644 /app/certs/tunnel.crt /app/certs/ca.crt

# Clean up (do NOT delete ca.key)
rm /app/certs/tunnel.csr /app/certs/ca.srl /app/certs/openssl.conf