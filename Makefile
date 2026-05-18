BINARY  := tokenmeter
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

COLLECTOR_DIR := deploy/collector
DEV_CONFIG    := config.dev.yaml
DEV_PID_FILE  := /tmp/tokenmeter-dev.pid
DEV_LOG_FILE  := /tmp/tokenmeter-dev.log

.PHONY: build test test-race lint clean release \
        dev-up dev-down dev-proxy dev-proxy-stop dev-logs dev-query dev-status \
        collector-up collector-down collector-logs collector-open \
        smoke

# ── Build ────────────────────────────────────────────────────────────────────

build:
	go build $(LDFLAGS) -o bin/$(BINARY) ./cmd/tokenmeter

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	go vet ./...
	go build ./...

clean:
	rm -rf bin/ dist/

release:
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  ./cmd/tokenmeter
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  ./cmd/tokenmeter
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   ./cmd/tokenmeter
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   ./cmd/tokenmeter
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/tokenmeter

# ── Collector (Docker) ───────────────────────────────────────────────────────

## Start OTel Collector + Prometheus + Grafana in the background
collector-up:
	docker compose -f $(COLLECTOR_DIR)/docker-compose.yaml up -d
	@echo ""
	@echo "  Collector stack running:"
	@echo "    OTLP gRPC  → localhost:4317"
	@echo "    Prometheus → http://localhost:9090"
	@echo "    Grafana    → http://localhost:3000  (admin / tokenmeter)"
	@echo ""

## Stop and remove collector containers (data volumes preserved)
collector-down:
	docker compose -f $(COLLECTOR_DIR)/docker-compose.yaml down

## Tail OTel Collector logs (shows every metric received)
collector-logs:
	docker compose -f $(COLLECTOR_DIR)/docker-compose.yaml logs -f otel-collector

## Open Grafana dashboard in the default browser
collector-open:
	@open http://localhost:3000 2>/dev/null || xdg-open http://localhost:3000 2>/dev/null || \
	  echo "Open http://localhost:3000 in your browser (admin / tokenmeter)"

# ── Edge proxy (local binary) ─────────────────────────────────────────────────

## Build and start the edge proxy in the background (writes to /tmp/tokenmeter-dev.log)
dev-proxy: build
	@if [ -f $(DEV_PID_FILE) ] && kill -0 $$(cat $(DEV_PID_FILE)) 2>/dev/null; then \
	  echo "Proxy already running (PID $$(cat $(DEV_PID_FILE))). Run 'make dev-proxy-stop' first."; \
	  exit 1; \
	fi
	@ANTHROPIC_BASE_URL="" OPENAI_BASE_URL="" \
	  nohup ./bin/$(BINARY) start --config $(DEV_CONFIG) > $(DEV_LOG_FILE) 2>&1 & \
	  echo $$! > $(DEV_PID_FILE); \
	  sleep 1; \
	  PID=$$(cat $(DEV_PID_FILE)); \
	  if kill -0 $$PID 2>/dev/null; then \
	    echo "Proxy started (PID $$PID)"; \
	    echo "  Listening: 127.0.0.1:4191"; \
	    echo "  Logs:      $(DEV_LOG_FILE)"; \
	    echo "  DB:        /tmp/tokenmeter-dev.db"; \
	  else \
	    echo "Proxy failed to start — check $(DEV_LOG_FILE)"; cat $(DEV_LOG_FILE); exit 1; \
	  fi

## Stop the background edge proxy (also clears any stale process on the listen port)
dev-proxy-stop:
	@if [ -f $(DEV_PID_FILE) ]; then \
	  PID=$$(cat $(DEV_PID_FILE)); \
	  if kill -0 $$PID 2>/dev/null; then \
	    kill $$PID && echo "Proxy stopped (PID $$PID)"; \
	  else \
	    echo "Proxy not running"; \
	  fi; \
	  rm -f $(DEV_PID_FILE); \
	else \
	  echo "No PID file found"; \
	fi
	@# Kill any stale process still bound to the listen port
	@STALE=$$(lsof -ti :4191 2>/dev/null); \
	  if [ -n "$$STALE" ]; then kill $$STALE 2>/dev/null && echo "Cleared stale process on :4191"; fi

## Tail the edge proxy log
dev-logs:
	@tail -f $(DEV_LOG_FILE)

## Query recent events from the dev SQLite database
dev-query:
	./bin/$(BINARY) query --config $(DEV_CONFIG) --last 1h

## Show proxy + collector status
dev-status:
	@echo "=== Edge proxy ==="
	@if [ -f $(DEV_PID_FILE) ] && kill -0 $$(cat $(DEV_PID_FILE)) 2>/dev/null; then \
	  echo "  Running (PID $$(cat $(DEV_PID_FILE)))"; \
	else \
	  echo "  Stopped"; \
	fi
	@echo ""
	@echo "=== Collector containers ==="
	@docker compose -f $(COLLECTOR_DIR)/docker-compose.yaml ps 2>/dev/null || echo "  Docker not running or collector not started"

# ── Full dev environment ───────────────────────────────────────────────────

## Start everything: collector stack + edge proxy
dev-up: collector-up dev-proxy
	@echo ""
	@echo "Full dev environment running."
	@echo ""
	@echo "Point any LLM tool at the proxy:"
	@echo "  export ANTHROPIC_BASE_URL=http://127.0.0.1:4191"
	@echo "  export OPENAI_BASE_URL=http://127.0.0.1:4191"
	@echo ""
	@echo "Then query captured events:"
	@echo "  make dev-query"
	@echo "  make collector-open"
	@echo ""

## Stop everything
dev-down: dev-proxy-stop collector-down
	@echo "Dev environment stopped."

# ── Smoke test ───────────────────────────────────────────────────────────────

## Full integration smoke test: start proxy, send a synthetic event, verify SQLite, stop.
## Does not require a live API key — uses a mock HTTP server via curl against the proxy health endpoint.
smoke: build
	@echo "--- smoke: starting proxy ---"
	@$(MAKE) dev-proxy-stop 2>/dev/null; true
	@$(MAKE) dev-proxy
	@echo "--- smoke: checking proxy health ---"
	@curl -sf http://127.0.0.1:4191/health > /dev/null && echo "  health: OK" || (echo "  health: FAIL"; $(MAKE) dev-proxy-stop; exit 1)
	@echo "--- smoke: done ---"
	@$(MAKE) dev-proxy-stop
