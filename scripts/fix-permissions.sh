#!/bin/sh

# Ensure Go cache directories have correct permissions
mkdir -p /go/pkg/mod /go/cache
chmod -R 777 /go/pkg/mod
chmod -R 777 /go/cache

# Execute the command passed to docker run
exec "$@"