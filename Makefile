# Environment files
ENV_DIR=internal/config/env
DEV_ENV=$(ENV_DIR)/.env.development
PROD_ENV=$(ENV_DIR)/.env.production

# Include all makefiles
include makefiles/env.mk
include makefiles/db/common.mk
include makefiles/db/dev.mk
include makefiles/db/prod.mk
include makefiles/server.mk
include makefiles/proto.mk
include makefiles/help.mk
include makefiles/release.mk

# Default target
.DEFAULT_GOAL := help
