# Repository Guidelines

## Project Structure & Module Organization

Hermes Tunnel is a Go CLI project. The executable entrypoint is in `cmd/hermes/main.go`. CLI commands live in `internal/cli`, built with Cobra. Runtime modules are split by responsibility: `internal/client` for the local tunnel client, `internal/gateway` for the public gateway and control server, `internal/protocol` for shared wire messages and HTTP helpers, and `internal/routing` for tunnel name routing. Tests sit beside the code they cover, for example `internal/routing/routing_test.go` and `internal/gateway/integration_test.go`. Development tooling is configured in `.air.toml`; generated binaries and temporary files should stay under `tmp/`, `bin/`, or `dist/` and are ignored by Git.

## Build, Test, and Development Commands

- `go mod tidy`: sync module dependencies and update `go.sum`.
- `go test ./...`: run all unit and integration tests.
- `go build ./cmd/hermes`: compile the CLI entrypoint.
- `go run ./cmd/hermes version`: run a quick CLI smoke test.
- `go run ./cmd/hermes server --public :8080 --control :8081 --token dev-secret`: start the public gateway and control listener.
- `go run ./cmd/hermes connect --name app --local http://localhost:3000 --server 127.0.0.1:8081 --token dev-secret`: expose a local HTTP app through the gateway.
- `air`: run the configured live-reload build target during development.

## Coding Style & Naming Conventions

Use standard Go formatting with `gofmt`. Keep package names short, lowercase, and domain-focused, such as `cli` and `tunnel`. Export only APIs needed across packages; keep protocol helpers and routing internals unexported. Prefer clear Go names like `ServerConfig`, `ClientConfig`, and `validateTunnelName`. Keep comments concise and reserved for behavior that is not obvious from the code.

## Testing Guidelines

Use Go's built-in `testing` package. Name tests with `Test...` and place them in `*_test.go` files next to the implementation. Add integration coverage when behavior crosses the server/client boundary, especially routing, request forwarding, headers, status codes, and shutdown behavior. Run `go test ./...` before submitting changes.

## Commit & Pull Request Guidelines

This repository has no existing commit history, so there is no established local convention yet. Use short, imperative commit subjects, for example `Add tunnel round-trip integration test` or `Validate client registration token`. Pull requests should include a brief summary, test results, linked issues when applicable, and notes for user-visible CLI or protocol changes.

## Security & Configuration Tips

Do not commit real tunnel tokens, domains, or private keys. Use `--token` for local testing and document any new configuration flags in `README.md`. Treat the current JSON-over-TCP protocol as development-grade until TLS, authentication hardening, and streaming limits are implemented.
