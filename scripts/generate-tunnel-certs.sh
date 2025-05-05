#!/bin/sh

# Create directory for certificates
mkdir -p /app/certs

# Create config file for server certificate
cat > /app/certs/server.conf << EOF
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
extendedKeyUsage = serverAuth

[alt_names]
DNS.1 = tunnel.giraffecloud.xyz
DNS.2 = *.tunnel.giraffecloud.xyz
EOF

# Create config file for client certificate
cat > /app/certs/client.conf << EOF
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
CN = giraffecloud-client

[req_ext]
basicConstraints = CA:FALSE
keyUsage = digitalSignature,keyEncipherment
extendedKeyUsage = clientAuth

[v3_ext]
basicConstraints = CA:FALSE
keyUsage = digitalSignature,keyEncipherment
extendedKeyUsage = clientAuth
EOF

# Generate CA private key and certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout /app/certs/ca.key -out /app/certs/ca.crt \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud/CN=GiraffeCloud Tunnel CA" \
  -addext "basicConstraints=critical,CA:true" \
  -addext "keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign"

# Generate server private key and CSR
openssl genrsa -out /app/certs/tunnel.key 4096
openssl req -new -key /app/certs/tunnel.key \
  -out /app/certs/tunnel.csr \
  -config /app/certs/server.conf

# Generate server certificate
openssl x509 -req -in /app/certs/tunnel.csr \
  -CA /app/certs/ca.crt -CAkey /app/certs/ca.key -CAcreateserial \
  -out /app/certs/tunnel.crt -days 365 \
  -extfile /app/certs/server.conf -extensions v3_ext

# Generate client private key and CSR
openssl genrsa -out /app/certs/client.key 4096
openssl req -new -key /app/certs/client.key \
  -out /app/certs/client.csr \
  -config /app/certs/client.conf

# Generate client certificate
openssl x509 -req -in /app/certs/client.csr \
  -CA /app/certs/ca.crt -CAkey /app/certs/ca.key -CAcreateserial \
  -out /app/certs/client.crt -days 365 \
  -extfile /app/certs/client.conf -extensions v3_ext

# Set proper permissions
chmod 600 /app/certs/tunnel.key /app/certs/client.key
chmod 644 /app/certs/tunnel.crt /app/certs/client.crt /app/certs/ca.crt

# Clean up
rm /app/certs/tunnel.csr /app/certs/client.csr /app/certs/ca.key /app/certs/ca.srl /app/certs/server.conf /app/certs/client.conf