# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

adngine is an ad-serving API. Advertisers register **conversations** (`internal/conversation`) - pieces of ad
content with a type (`knowledge`, `action`, `evaluation`), an optional related product (e.g. `veiculo`,
`eletrodomestico`), display priority, and the list of display components they're eligible for. Given a client and a
set of requested display components (slots), the API returns the single best conversation for each slot.

Conversations are currently registered via a static YAML file (`configs/conversations.yaml`), not a database -
this is intentional for the MVP stage.

## Commands

```bash
# build / vet everything
go build ./...
go vet ./...

# run the API (reads configs/config.yaml by default)
go run ./cmd/adngine
go run ./cmd/adngine -config path/to/config.yaml

# add/update dependencies after changing imports
go mod tidy

# run all tests / a single package / a single test
go test ./...
go test ./internal/selection/...
go test ./internal/selection/ -run TestBannerSelector
```

Tests live alongside the code they cover, in external test packages (`package selection_test`). Shared
conversation inventories used by tests live in `internal/testsupport/fixtures/` and are loaded through
`testsupport.LoadRepository(t, "inventory_valid.yaml")`, which resolves the path from the `testsupport`
package itself - so any package can use the same fixtures.

## Architecture

Request flow: `cmd/adngine` -> `internal/app` (wiring) -> `internal/httpserver` (Fiber routes/handlers) ->
`internal/selection` (Registry) -> `internal/conversation` (Repository + domain model).

- **`internal/app`** - the application's single init/wiring point (`app.New`). Loads config, builds the logger,
  loads the conversation repository, builds the selection registry, and constructs the HTTP server. `cmd/adngine`
  only parses the `-config` flag and calls into this package; it has no other logic.
- **`internal/config`** - Viper-based config loading (`config.Load(path)`) into a typed `Config` struct
  (`server.port`, `log.level`, `conversations.file_path`).
- **`internal/conversation`** - the domain model (`Conversation`, `Type`, `Client`) and the `Repository`, which
  loads the conversation registry from YAML (also via Viper) and answers `ByComponent(component)` queries.
- **`internal/selection`** - the selection engine. `Selector` is the per-component interface; **each component
  (banner, card, footer) has its own file** (`banner.go`, `card.go`, `footer.go`) with its own selector type, so
  a component's ranking rule can be changed without touching the others. All three currently delegate to the
  shared `bestMatch` helper in `selector.go` (filter candidates by product match, pick highest `Priority`) - when a
  component needs bespoke logic, change only that component's `Select` method. `Registry` (also in `selector.go`)
  maps component name -> `Selector` and is the only thing `internal/httpserver` talks to.
- **`internal/httpserver`** - Fiber app setup and the `POST /v1/selections` handler. Request/response shapes live
  in `dto.go`; the handler never exposes `conversation.Conversation` directly.

### Adding a new display component

1. Add a new file in `internal/selection` (e.g. `sidebar.go`) with its own `Component<Name>` constant and
   `<Name>Selector` type implementing `Selector`.
2. Register it in `NewRegistry` in `selector.go`.
3. Reference the new component name in `configs/conversations.yaml` under a conversation's `components` list.

### Adding a new conversation

Add an entry to `configs/conversations.yaml` (`id`, `type`, `product`, `text`, `link`, `priority`, `components`).
No code change is needed unless the entry requires new selection logic.

## Stack

Go, `github.com/gofiber/fiber/v2` for HTTP, `log/slog` (stdlib) for logging, `github.com/spf13/viper` for both
app config and the conversation registry file.
