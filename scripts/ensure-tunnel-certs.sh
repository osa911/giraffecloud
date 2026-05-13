#!/bin/sh
set -e

# Ensure tunnel TLS material is present and not expiring soon.
# - CA: regenerate only if missing or expiring within CA_RENEW_BEFORE (default 90 days).
# - Leaf: regenerate if missing, expiring within LEAF_RENEW_BEFORE (default 30 days),
#         or if the CA was just rotated (the old leaf would chain to a stale CA).
#
# Regenerating the CA invalidates every client cert issued against it, so the
# threshold is intentionally generous to give operators time to plan re-logins.

CERT_DIR="${CERT_DIR:-/app/certs}"
CA_CRT="$CERT_DIR/ca.crt"
LEAF_CRT="$CERT_DIR/tunnel.crt"
LEAF_KEY="$CERT_DIR/tunnel.key"
SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

CA_RENEW_BEFORE="${CA_RENEW_BEFORE:-$((90 * 24 * 3600))}"
LEAF_RENEW_BEFORE="${LEAF_RENEW_BEFORE:-$((30 * 24 * 3600))}"

mkdir -p "$CERT_DIR"

need_ca=0
if [ ! -f "$CA_CRT" ]; then
  need_ca=1
  echo "Tunnel CA missing — will generate."
elif ! openssl x509 -in "$CA_CRT" -noout -checkend "$CA_RENEW_BEFORE" >/dev/null 2>&1; then
  need_ca=1
  echo "Tunnel CA expires within $((CA_RENEW_BEFORE / 86400)) days — will rotate (invalidates all client certs)."
fi

need_leaf=0
if [ "$need_ca" -eq 1 ]; then
  need_leaf=1
elif [ ! -f "$LEAF_CRT" ] || [ ! -f "$LEAF_KEY" ]; then
  need_leaf=1
  echo "Tunnel leaf cert missing — will generate."
elif ! openssl x509 -in "$LEAF_CRT" -noout -checkend "$LEAF_RENEW_BEFORE" >/dev/null 2>&1; then
  need_leaf=1
  echo "Tunnel leaf cert expires within $((LEAF_RENEW_BEFORE / 86400)) days — will rotate."
fi

if [ "$need_ca" -eq 1 ]; then
  "$SCRIPT_DIR/generate-tunnel-ca.sh"
fi

if [ "$need_leaf" -eq 1 ]; then
  "$SCRIPT_DIR/generate-tunnel-leaf.sh"
fi

if [ "$need_ca" -eq 0 ] && [ "$need_leaf" -eq 0 ]; then
  leaf_expiry="$(openssl x509 -in "$LEAF_CRT" -noout -enddate 2>/dev/null | cut -d= -f2)"
  echo "Tunnel certs valid — no action needed (leaf expires: $leaf_expiry)"
fi
