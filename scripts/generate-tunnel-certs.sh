#!/bin/sh

# Create directory for certificates
mkdir -p /app/certs

# Generate CA private key and certificate
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout /app/certs/ca.key -out /app/certs/ca.crt \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud/CN=Tunnel CA"

# Generate server private key
openssl genrsa -out /app/certs/tunnel.key 4096

# Generate server CSR
openssl req -new -key /app/certs/tunnel.key \
  -out /app/certs/tunnel.csr \
  -subj "/C=US/ST=State/L=City/O=GiraffeCloud/CN=tunnel.giraffecloud.xyz"

# Generate server certificate
openssl x509 -req -in /app/certs/tunnel.csr \
  -CA /app/certs/ca.crt -CAkey /app/certs/ca.key -CAcreateserial \
  -out /app/certs/tunnel.crt -days 365

# Set proper permissions
chmod 600 /app/certs/tunnel.key
chmod 644 /app/certs/tunnel.crt
chmod 644 /app/certs/ca.crt

# Clean up
rm /app/certs/tunnel.csr /app/certs/ca.key /app/certs/ca.srl