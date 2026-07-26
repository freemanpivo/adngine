GO      ?= go
BINARY  := adngine
BIN_DIR := bin
CONFIG  ?= configs/config.yaml
PORT    ?= 8080

# PKG e RUN permitem mirar um pacote ou um teste: make test PKG=./internal/selection/... RUN=TestRegistry
PKG ?= ./...
RUN ?=
RUN_FLAG := $(if $(RUN),-run $(RUN),)

SMOKE_BODY ?= {"client":{"id":"cliente-123","product":"veiculo"},"slots":["banner","card","footer"]}

.DEFAULT_GOAL := help

.PHONY: help
help: ## Lista os alvos disponiveis
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Sobe a API (CONFIG=configs/config.yaml)
	$(GO) run ./cmd/adngine -config $(CONFIG)

.PHONY: build
build: ## Compila o binario em bin/
	$(GO) build -o $(BIN_DIR)/$(BINARY) ./cmd/adngine

.PHONY: test
test: ## Roda os testes (PKG=... RUN=...)
	$(GO) test $(RUN_FLAG) $(PKG)

.PHONY: test-race
test-race: ## Roda os testes com o detector de corrida, sem cache
	$(GO) test -race -count=1 $(RUN_FLAG) $(PKG)

.PHONY: test-integration
test-integration: ## Roda os testes marcados com a tag integration (dependencias externas no ar)
	$(GO) test -tags=integration -count=1 $(RUN_FLAG) $(PKG)

.PHONY: cover
cover: ## Roda os testes com cobertura e imprime o total por pacote
	@$(GO) test -coverprofile=$(BIN_DIR)/coverage.out $(PKG) > /dev/null
	@$(GO) tool cover -func=$(BIN_DIR)/coverage.out | tail -20

.PHONY: cover-html
cover-html: cover ## Abre o relatorio de cobertura no navegador
	@$(GO) tool cover -html=$(BIN_DIR)/coverage.out

.PHONY: fmt
fmt: ## Formata o codigo
	gofmt -w .

.PHONY: fmt-check
fmt-check: ## Falha se houver arquivo fora do formato
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then echo "arquivos fora do formato:"; echo "$$out"; exit 1; fi

.PHONY: vet
vet: ## Roda o go vet
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Sincroniza go.mod e go.sum
	$(GO) mod tidy

.PHONY: check
check: fmt-check vet test-race ## Portao completo: formato, vet e testes com -race

.PHONY: smoke
smoke: build ## Sobe a app, faz uma requisicao de exemplo e derruba
	@mkdir -p $(BIN_DIR)
	@if command -v lsof > /dev/null && lsof -nP -iTCP:$(PORT) -sTCP:LISTEN > /dev/null 2>&1; then \
		echo "porta $(PORT) ja esta em uso; o smoke conversaria com outro processo"; exit 1; \
	fi
	@./$(BIN_DIR)/$(BINARY) -config $(CONFIG) > $(BIN_DIR)/smoke.log 2>&1 & echo $$! > $(BIN_DIR)/smoke.pid; \
	trap 'kill $$(cat $(BIN_DIR)/smoke.pid) 2>/dev/null; rm -f $(BIN_DIR)/smoke.pid' EXIT; \
	ready=0; \
	for _ in $$(seq 1 50); do \
		if curl -sf -o /dev/null -X POST http://localhost:$(PORT)/v1/selections \
			-H 'Content-Type: application/json' -d '$(SMOKE_BODY)'; then ready=1; break; fi; \
		sleep 0.1; \
	done; \
	if [ $$ready -eq 0 ]; then echo "app nao respondeu na porta $(PORT):"; cat $(BIN_DIR)/smoke.log; exit 1; fi; \
	printf '> POST /v1/selections %s\n' '$(SMOKE_BODY)'; \
	curl -s -X POST http://localhost:$(PORT)/v1/selections \
		-H 'Content-Type: application/json' -d '$(SMOKE_BODY)' | python3 -m json.tool; \
	echo "> log de boot em $(BIN_DIR)/smoke.log"

.PHONY: clean
clean: ## Remove os artefatos de build
	rm -rf $(BIN_DIR)
