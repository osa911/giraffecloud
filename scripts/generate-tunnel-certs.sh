#!/bin/sh
set -e

# Deprecated entry point — kept as a shim so any external callers (docs, ops
# runbooks, ad-hoc shell invocations) keep working. The real logic lives in:
#   - generate-tunnel-ca.sh     (CA only, long-lived)
#   - generate-tunnel-leaf.sh   (leaf signed by existing CA)
#   - ensure-tunnel-certs.sh    (validity-aware orchestrator — preferred)
#
# This shim delegates to ensure-tunnel-certs.sh, which is idempotent: it only
# regenerates material that is missing or expiring soon. To force a full
# rebuild, delete /app/certs/* first.

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
exec "$SCRIPT_DIR/ensure-tunnel-certs.sh" "$@"
