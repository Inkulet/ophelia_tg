.SHELLFLAGS := -eu -o pipefail -c
SHELL := /bin/bash
.PHONY: up frontend-build backend-run

FRONTEND_DIR := frontend
WEB_ADDR ?= :8080

up: frontend-build backend-run

frontend-build:
	@NODE_MAJOR=$$(node -p "process.versions.node.split('.')[0]" 2>/dev/null || echo 0); \
	if (( NODE_MAJOR % 2 == 1 )); then \
		echo "Detected non-LTS Node.js $$NODE_MAJOR.x. Trying to use LTS via nvm..."; \
		export NVM_DIR="$$HOME/.nvm"; \
		if [[ -s "$$NVM_DIR/nvm.sh" ]]; then \
			source "$$NVM_DIR/nvm.sh"; \
			nvm install --lts >/dev/null; \
			nvm use --lts >/dev/null; \
		else \
			echo "nvm is not available. Please install/use Node.js LTS (20.x or 22.x) and re-run make up."; \
			exit 1; \
		fi; \
	fi; \
	cd $(FRONTEND_DIR) && npm ci && npm run build -- --prerender=false

backend-run:
	OPHELIA_WEB_ADDR=$(WEB_ADDR) go run .
