# Linkforge

A URL shortener with planned real-time click analytics, built incrementally as the course project for **Golang for developers**.

The design is specified in the project TDD. The repository contains the implemented domain service and in-memory storage, alongside contracts for later milestones.

## Current state

Homework 4 extends the service and CLI from Homeworks 1–3 with reusable data structures and an optional service cache:

- `internal/base62` provides encoding, recursive encoding, padding, and decoding with overflow checks.
- `link.Service` implements `Shortener`: it creates links with generated codes or custom aliases, resolves links while checking expiry, lists an owner's links with cursor pagination, and checks ownership before deletion.
- `store.Store` implements `link.Store` using maps indexed by ID and code. It rejects duplicate IDs and codes and returns lists ordered by ID.
- The service owns the ID counter and retries generated-code collisions up to three attempts. `WithClock` supplies a controllable clock for tests.
- Sentinel errors and `AliasError` support inspection with `errors.Is` and `errors.As`.
- `internal/config` loads and validates environment variables, joins configuration errors, and redacts credentials in structured logs.
- `internal/linkio` reads and writes JSON Lines and exports CSV with UTC timestamps and proper quoting.
- `linkctl` provides `import`, `export`, `config`, and `version` commands. The JSON Lines file preserves links between CLI runs.
- `cache.LRU[K,V]` combines a map with `container/list` to keep a bounded cache in most-to-least-recently-used order.
- `trie.Trie` indexes words by byte and returns prefix matches in ascending byte order, with an optional result limit.
- `topk.TopK` uses `container/heap` to retain the highest counts, breaking ties by the smaller key. Duplicate keys remain separate entries.
- `shortid.CodeGenerator` combines 19 random bits from an injected reader with a 16-bit counter to produce six-character base62 codes. The service still generates its codes from IDs.
- `link.WithCache` accepts the consumer-owned `link.Cache` interface. `Resolve` checks context first, uses cache hits without reading the store, fills misses after successful store reads, and checks expiry in both cases. Successful deletion invalidates the cache; `Shorten` and `List` leave it alone. The service also works without a cache.

Tests cover the new data structures, code generation, cache integration, and regressions from Homeworks 1–3. See [Validation](#validation), [Complexity](#complexity), and [Benchmarks](#benchmarks).

The original interfaces and `Noop*` implementations remain in place. The new mutable library types are not safe for concurrent use. The demo uses the concrete service and store; its storage lasts for the current process and is intended for sequential use at this stage.

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

The invalid address is reported through structured logging to standard error. The log includes a timestamp and these fields:

```text
level=ERROR msg="shorten failed" argument=2 error="link: error unsupported scheme"
```

The command creates links for owner `demo`, continues after invalid input, and prints the stored links using `All` and `Count`. An invalid request does not consume an ID.

## Environment configuration

`linkctl config` validates configuration from the environment. Values are trimmed; unset, empty, and whitespace-only variables use the defaults below. Required values have no default.

| Variable | Default | Validation |
|---|---|---|
| `LF_HTTP_ADDR` | `:8080` | `host:port`; numeric port from 1 to 65535 |
| `LF_ADMIN_ADDR` | `127.0.0.1:6060` | `host:port`; numeric port from 1 to 65535 |
| `LF_TCP_ADDR` | `:7070` | `host:port`; numeric port from 1 to 65535 |
| `LF_PG_DSN` | Required | Nonempty; no connection or DSN parsing |
| `LF_MONGO_URI` | Required | Nonempty; no connection or URI parsing |
| `LF_JWT_SECRET` | Required | At least 32 bytes after trimming |
| `LF_CACHE_SIZE` | `100000` | Positive integer |
| `LF_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`, case-insensitive |

For example, using local demonstration values:

```bash
LF_PG_DSN='local-demo-dsn' \
LF_MONGO_URI='local-demo-uri' \
LF_JWT_SECRET='development-only-secret-at-least-32-bytes' \
go run ./cmd/linkctl config
```

Success prints `config ok` to stdout. Configuration logs replace the three credentials with `[redacted]`. Invalid configuration returns all validation problems together and exit code 1. An invalid `LF_LOG_LEVEL` is a CLI usage error and returns code 2.

The `import`, `export`, and `version` commands do not require database credentials or a JWT secret. They only read `LF_LOG_LEVEL` for logging.

## Import and export

The input file contains one target URL per line. Blank lines and comments starting with `#` after trimming are skipped. Surrounding whitespace is removed from URLs. Both LF and CRLF line endings are supported, and the final line does not need a newline:

```bash
printf 'https://example.com\nhttps://go.dev\n' > urls.txt
go run ./cmd/linkctl import -in urls.txt -db links.jsonl
go run ./cmd/linkctl export -db links.jsonl
```

The import prints:

```text
imported: 2
skipped: 0
```

`links.jsonl` is the database: a plain file with one JSON-encoded `link.Link` per line. Import loads existing links, assigns new IDs after the current maximum, and saves the complete list. A missing database file starts an empty database. Imported links belong to owner `cli`.

Export writes CSV to stdout by default. To write a file instead:

```bash
go run ./cmd/linkctl export -db links.jsonl -out links.csv
```

The CSV header is `id,code,target,owner_id,created_at,expires_at`. Timestamps use RFC 3339 in UTC; links without an expiry have an empty `expires_at`. An empty database exports just the header.

Invalid targets are counted as skipped while valid targets are saved. Import then returns code 1 if any targets were skipped. Cancellation before saving leaves the database unchanged. Program exit codes are 0 for success, 1 for operational failure or skipped targets, and 2 for incorrect command usage. With `go run`, Go also prints `exit status N` when the program exits unsuccessfully.

Results and CSV go to stdout; usage messages, errors, and structured `slog` logs go to stderr. Import logs its start and finish at `info`, skipped targets at `warn` with `line` and `error`, and imported targets at `debug`.

```bash
LF_LOG_LEVEL=debug go run ./cmd/linkctl import -in urls.txt -db links.jsonl
go run -ldflags '-X main.version=1.0.0' ./cmd/linkctl version
```

## Layout

```
cmd/      linkforge         CLI demo using the service and in-memory store
          linkctl           import, export, config validation, and version

pkg/                        reusable libraries — a separate Go module
  shortid/  Codec           base62 codec contract (placeholder)        M1
            Generator       code generator contract                    M4
            CodeGenerator   random bits + counter, six base62 chars    M4
  cache/    Cache[K,V]       cache contract                             M4
            LRU[K,V]        bounded map + doubly linked list           M4
  trie/     Trie            byte-based prefix index                    M4
  topk/     TopK, Item      bounded heap of ranked items               M4

internal/                   application code
  base62/                   base62 codec and recursive encoder         M1 / M2
  link/     Store           link persistence, declared by its consumer M1 / M7
            Shortener       the domain service behind every transport  M2
            Service         Shortener with optional WithCache          M2 / M4
            Cache           lookup cache, declared by its consumer     M4
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
  linkio/                   text input, JSON Lines storage, CSV export M3
  observability/Metrics     counters and histograms                    M10
```

The `pkg/` tree is a second module so that reusable libraries stay free of any dependency on application code; `go.work` ties the two together for local development. The libraries use only the standard library. `internal/link` imports `pkg/cache` only in tests, where `*cache.LRU[string, link.Link]` satisfies `link.Cache` without an adapter.

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

## Complexity

Here, `n` is the number of cached entries, `N` is the number of retained heap items, `m` is the inserted word's byte length, `L` is the prefix's byte length, and `k` is the number of returned words. For trie traversal, `v` counts visited nodes below the prefix and `D` is the maximum byte length of a path string constructed during the search. Hashing and comparing cache or heap keys is treated as constant time; longer keys add their hashing or comparison cost.

| Operation | Average / amortized time | Worst-case time |
|---|---|---|
| `LRU.Get` | `O(1)` | `O(n)` |
| `LRU.Put` | `O(1)` | `O(n)` |
| `Trie.Insert` | `O(m)` | `O(m)` |
| `Trie.WithPrefix` | `O(L + v·D + k)` | `O(L + v·D + k)` |
| `TopK.Add` | `O(log N)` | `O(log N)` |
| `TopK.Result` | `O(N log N)` | `O(N log N)` |

`LRU.Get` uses the map to find a `*list.Element` in average `O(1)` time. Moving that known element to the front of the doubly linked list changes a fixed number of pointers, so the combined operation remains average `O(1)`. The `O(n)` worst-case bounds account for pathological map collisions and, for insertion, map growth; list operations themselves remain `O(1)`.

`Trie.WithPrefix` first follows the `L` prefix bytes, then visits children in byte order until it has `k` results or exhausts the subtree. Each node has 256 child slots, a constant factor, but this implementation also copies the accumulated path string on each descent, costing up to `O(D)` per visited node; collecting the results costs `O(k)` overall. Deleted words leave their paths in the trie, so the search can visit nodes that produce no result, and its cost cannot be expressed using only `L` and `k`.

`TopK.Add` uses a heap push, or a pop followed by a push when replacing the worst retained item. A full heap rejects an item that is not better in `O(1)`; insertion and replacement are bounded by `O(log N)`. `Result` copies the retained items and sorts the copy in `O(N log N)` time, leaving the heap unchanged. Empty and single-item heaps take constant time.

## Benchmarks

Run the benchmarks in the library module from the repository root:

```bash
make bench
```

`BenchmarkLRUGetHit` cycles through 1,024 cached six-character string keys. Every lookup is a hit and moves its entry to the front of the LRU list. `BenchmarkTrieWithPrefix` builds 10,000 words (`alias-0000` through `alias-9999`) and queries `alias-0`, which matches 1,000 words. Its two cases return the first 10 matches or all matches. Both benchmarks use `b.Loop()` and `b.ReportAllocs()`; data preparation is outside the measured loop.

Recorded on 2026-11-13 with Go 1.27.2, macOS/arm64, and an Apple M2:

```text
cd pkg && go test -run '^$' -bench . -benchmem ./...
goos: darwin
goarch: arm64
pkg: github.com/al-bo/rd-linkforge/pkg/cache
cpu: Apple M2
BenchmarkLRUGetHit-8                       35755564         33.25 ns/op         0 B/op         0 allocs/op
PASS
ok      github.com/al-bo/rd-linkforge/pkg/cache       1.482s
PASS
ok      github.com/al-bo/rd-linkforge/pkg/shortid     0.242s
PASS
ok      github.com/al-bo/rd-linkforge/pkg/topk        0.228s
goos: darwin
goarch: arm64
pkg: github.com/al-bo/rd-linkforge/pkg/trie
cpu: Apple M2
BenchmarkTrieWithPrefix/limit_10-8           418609          3291 ns/op       680 B/op        17 allocs/op
BenchmarkTrieWithPrefix/unlimited-8           3248        354866 ns/op     52864 B/op      1121 allocs/op
PASS
ok      github.com/al-bo/rd-linkforge/pkg/trie        2.813s
```

The LRU hit performs no allocations; limiting the trie result to 10 words reduces this run from 1,121 allocations and about 355 microseconds for 1,000 matches to 17 allocations and about 3.3 microseconds. Timings depend on the machine and run.

## Validation

Run the Homework 4 checks from the repository root:

```bash
make check build
gofmt -l .
go test -cover ./...
(cd pkg && go test -cover ./...)
make bench
```

`make check build` runs vet, lint, tests with the race detector, and builds in both modules. `gofmt -l .` should produce no output. The root module's `go test ./...` and `make cover` do not include the nested `pkg` module, so its coverage must be checked separately.

Homework 4 requires at least 85% statement coverage in each new or extended library package. Recorded on 2026-11-13 with `cd pkg && go test -cover ./...`:

| Package | Minimum | Measured |
|---|---:|---:|
| `pkg/cache` | 85% | 90.3% |
| `pkg/trie` | 85% | 100.0% |
| `pkg/topk` | 85% | 100.0% |
| `pkg/shortid` | 85% | 88.9% |

Coverage requirements from Homework 3 still apply:

| Package | Minimum |
|---|---:|
| `internal/config` | 85% |
| `internal/linkio` | 80% |
| `cmd/linkctl` | 70% |

The tests use injected environment lookups, `t.TempDir()` for files, fixed readers for random bytes, and deterministic cancellation without `time.Sleep`. Test helpers call `t.Helper()`. The three original `hw2_test.go` files remain unchanged.

Library tests cover LRU eviction and recency, trie insertion/deletion and byte ordering, bounded heap ranking and independent result copies, and the generator's reference codes, read errors, and counter wraparound. Cache integration tests cover hits and misses, expiry, context cancellation, store errors, deletion ordering, operation without a cache, and a real LRU passed to `WithCache`.

Regression tests cover complete and fragmented input reads, empty input, whitespace and comments, CRLF, missing final newlines, and wrapped reader errors. Service and store methods check context before accessing data or validating requests. A canceled `Shorten` does not consume an ID or reach the store, and `WithFirstID(0)` leaves the counter unchanged.

## Milestones

| # | Lessons | Scope |
|---|---|---|
| M0 | 1 | Toolchain, module layout, CI |
| M1 | 2–3 | base62 codec, `Link` model, in-memory store |
| M2 | 4–5 | Methods, interfaces, generics, error values |
| M3 | 6–7 | Files and `context`, CLI, `slog`, tests |
| M4 | 8–9 | LRU, byte trie, top-N heap, code generator, cache integration, benchmarks |
| M5 | 10–11 | Ingestion pipeline, worker pool, synchronisation |
| M6 | 12–13 | TCP resolver, graceful shutdown, Docker |
| M7 | 14 | PostgreSQL and MongoDB stores |
| M8 | 15–16 | `net/http` first, then a framework; REST and Swagger |
| M9 | 17–19 | Validation, JWT and middleware, WebSockets |
| M10 | 20–21 | Production hardening and project defence |
