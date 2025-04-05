# Include all makefiles
include makefiles/env.mk
include makefiles/db.mk
include makefiles/server.mk
include makefiles/help.mk

# Default target
.DEFAULT_GOAL := help

# Environment files
ENV_DIR=internal/config/env
DEV_ENV=$(ENV_DIR)/.env.development
PROD_ENV=$(ENV_DIR)/.env.production
