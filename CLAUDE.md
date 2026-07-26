# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

adngine is an ad-serving API. Advertisers register **conversations** (`internal/conversation`) - pieces of ad
content with a type (`knowledge`, `action`, `evaluation`), an optional related product (e.g. `veiculo`,
`eletrodomestico`), display priority, and the list of display components they're eligible for. Given a client and a
set of requested display components (slots), the API returns the single best conversation for each slot.

Conversations are registered via static YAML files, one per display component (`configs/conversations/banner.yaml`,
`card.yaml`, `footer.yaml`), not a database - this is intentional for the MVP stage. Each file also declares the
component's `fallbacks`, one per product plus a mandatory default (`product: ""`).

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
  (`server.port`, `log.level`, `selection.global_timeout`, and `selection.components.<name>` with `file_path`,
  `timeout` and `max_calls`). Defaults are applied in code, not through Viper defaults: declaring any component in
  the config replaces the built-in list entirely, so removing a component from the config actually removes it.
- **`internal/conversation`** - the domain model (`Conversation`, `Type`, `Source`, `EligibilitySpec`, `Client`,
  `ComponentInventory`) and the `Repository`, which loads one inventory file per component (via Viper) and answers
  `Candidates(component)` and `Fallback(component, product)`. Both return copies, so consumers cannot corrupt the
  in-memory inventory. `validate.go` runs at load time and accumulates every violation instead of stopping at the
  first one.
- **`internal/selection`** - the selection engine. `Selector` is the per-component interface; **each component
  (banner, card, footer) has its own file** (`banner.go`, `card.go`, `footer.go`) with its own selector type, so
  a component's ranking rule can be changed without touching the others. All three currently delegate to the
  shared `bestMatch` helper in `selector.go` (filter candidates by product match, pick highest `Priority`) - when a
  component needs bespoke logic, change only that component's `Select` method. `Registry` (also in `selector.go`)
  maps component name -> `Selector` and is the only thing `internal/httpserver` talks to.
- **`internal/httpserver`** - Fiber app setup and the `POST /v1/selections` handler. Request/response shapes live
  in `dto.go`; the handler never exposes `conversation.Conversation` directly.

### Adding a new display component

1. Create `configs/conversations/<name>.yaml` with `component: <name>`, a `fallbacks` list (default entry
   required) and the conversations.
2. Add the component under `selection.components` in `configs/config.yaml` with its `file_path` and `timeout`.

That is enough: a component with no registered selector falls back to `DefaultSelector`. Only add a file in
`internal/selection` (e.g. `sidebar.go`) with its own `Component<Name>` constant and `<Name>Selector` type, and
register it in `selectorsByComponent` in `selector.go`, when the component needs a ranking rule of its own.

### Adding a new conversation

Add an entry to the component's file in `configs/conversations/` (`id`, `type`, `product`, `text`, `link`,
`priority`, and optionally `eligibility`). A conversation that should appear in two components is declared in both
files - the duplication is intentional, since the inventories are independent and may diverge. No code change is
needed unless the entry requires new selection logic.

## Stack

Go, `github.com/gofiber/fiber/v2` for HTTP, `log/slog` (stdlib) for logging, `github.com/spf13/viper` for both
app config and the conversation registry file.
