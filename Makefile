.PHONY: up frontend-build backend-run

FRONTEND_DIR := frontend
WEB_ADDR ?= :8080

up: frontend-build backend-run

frontend-build:
	cd $(FRONTEND_DIR) && npm ci && npm run build

backend-run:
	OPHELIA_WEB_ADDR=$(WEB_ADDR) go run .
