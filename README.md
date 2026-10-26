# Linkforge

A URL shortener with planned real-time click analytics, built incrementally as the course project for **Golang for developers**.

The design is specified in the project TDD. The repository contains the implemented domain service and in-memory storage, alongside contracts for later milestones.

## Current state

Homework 2 implements the `Shortener` service and connects it to an in-memory store:

- `internal/base62` provides encoding, recursive encoding, padding, and decoding with overflow checks.
- `link.Service` implements `Shortener`: it creates links with generated codes or custom aliases, resolves links while checking expiry, lists an owner's links with cursor pagination, and checks ownership before deletion.
- `store.Store` implements `link.Store` using maps indexed by ID and code. It rejects duplicate IDs and codes and returns lists ordered by ID.
- The service owns the ID counter and retries generated-code collisions up to three attempts. `WithClock` supplies a controllable clock for tests.
- Sentinel errors and `AliasError` support inspection with `errors.Is` and `errors.As`.

The original interfaces and `Noop*` implementations remain in place. The demo uses the concrete service and store. Storage lasts for the current process and is intended for sequential use at this stage.

## Run the demo

Requires Go 1.25 or newer. From the repository root:

```bash
go run ./cmd/linkforge https://example.com ftp://nope https://go.dev
```

Standard output:

```text
000001    https://example.com
000002    https://go.dev
Total: 2
```

The invalid address is reported to standard error:

```text
shorten "ftp://nope": link: error unsupported scheme
```

The command creates links for owner `demo`, continues after invalid input, and prints the stored links using `All` and `Count`. An invalid request does not consume an ID.

## Layout

```
cmd/      linkforge         CLI demo using the service and in-memory store
          linkctl           operator CLI   — prints its version, nothing more

pkg/                        reusable library contracts — a separate Go module
  shortid/  Codec           base62 codec contract (placeholder)        M1
            Generator       collision-resistant code issuing           M4
  cache/    Cache[K,V]      bounded cache on the redirect hot path     M4

internal/                   application code
  base62/                   base62 codec and recursive encoder         M1 / M2
  link/     Store           link persistence, declared by its consumer M1 / M7
            Shortener       the domain service behind every transport  M2
            Service         concrete Shortener implementation          M2
  store/    Store           in-memory link storage                     M1 / M2
  click/    Ingester        non-blocking submission from the hot path  M5
            Store           append-only raw event persistence          M7
            Aggregator      time series and top-N queries              M5
            Publisher       fan-out to live WebSocket subscribers      M9
  auth/     Issuer          access token minting                       M9
            Verifier        token validation for middleware            M9
            UserStore       account persistence                        M7
  transport/Server          uniform listener lifecycle                 M6 / M8
  config/   Loader          environment-only configuration             M3
  observability/Metrics     counters and histograms                    M10
```

The `pkg/` tree is a second module so that reusable libraries stay free of any dependency on application code; `go.work` ties the two together for local development.

## Design rules

These hold for every milestone.

1. **Interfaces are declared by the consumer.** `link.Store` lives in `internal/link` because the shortener is what needs it — not beside any implementation. Constructors return concrete types.
2. **The dependency arrow points inwards.** `internal/link` and `internal/click` may not import a transport or a storage package.
3. **Analytics never blocks a redirect.** `Ingester.Submit` drops events under saturation rather than applying backpressure.
4. **Every listener shares one lifecycle contract**, which is what makes a single coordinated graceful shutdown possible.

## Working on it

Run the application module's tests, including the homework tests, from the repository root:

```bash
go test ./...
```

Use `make test` to check both the application module and the separate `pkg` module with the race detector. Other development commands are:

```bash
make            # vet, lint and test — the same gates CI runs
make build
make test
make cover
make bench
make tidy
```

Linting uses [golangci-lint](https://golangci-lint.run) (`brew install golangci-lint`).

## Milestones

| # | Lessons | Scope |
|---|---|---|
| M0 | 1 | Toolchain, module layout, CI |
| M1 | 2–3 | base62 codec, `Link` model, in-memory store |
| M2 | 4–5 | Methods, interfaces, generics, error values |
| M3 | 6–7 | Files and `context`, CLI, `slog`, tests |
| M4 | 8–9 | Own hash table, LRU, trie, top-N heap |
| M5 | 10–11 | Ingestion pipeline, worker pool, synchronisation |
| M6 | 12–13 | TCP resolver, graceful shutdown, Docker |
| M7 | 14 | PostgreSQL and MongoDB stores |
| M8 | 15–16 | `net/http` first, then a framework; REST and Swagger |
| M9 | 17–19 | Validation, JWT and middleware, WebSockets |
| M10 | 20–21 | Production hardening and project defence |
