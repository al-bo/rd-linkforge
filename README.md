# Linkforge

A URL shortener with planned real-time click analytics, built incrementally as the course project for **Golang for developers**.

The design is specified in the project TDD. The repository contains a TCP resolver, an operator CLI, the domain service and in-memory storage, alongside contracts for later milestones.

## Current state

Homework 6 exposes the service over TCP. `linkforged` loads a JSON Lines database, resolves codes through a concurrent LRU cache, and shuts down on Ctrl+C or SIGTERM. Docker packaging remains unfinished; the commands below describe the intended container workflow once `Dockerfile` and `.dockerignore` are added.

- `internal/base62` provides encoding, recursive encoding, padding, and decoding with overflow checks.
- `link.Service` implements `Shortener`: it creates links with generated codes or custom aliases, resolves links while checking expiry, lists an owner's links with cursor pagination, and checks ownership before deletion.
- `store.Store` implements `link.Store` using maps indexed by ID and code. It rejects duplicate IDs and codes and returns lists ordered by ID.
- The service owns the ID counter and retries generated-code collisions up to three attempts. `WithClock` supplies a controllable clock for tests.
- Sentinel errors and `AliasError` support inspection with `errors.Is` and `errors.As`.
- `internal/config` loads and validates environment variables, joins configuration errors, and redacts credentials in structured logs.
- `internal/linkio` reads and writes JSON Lines and exports CSV with UTC timestamps and proper quoting.
- `linkctl` provides `import`, `export`, `config`, `version`, and `resolve` commands. The JSON Lines file preserves links between CLI runs.
- `internal/transport/tcp` implements `transport.Server`: one goroutine per connection, a 4,096-byte request limit, a 60-second idle timeout, and TCP keep-alive every 30 seconds.
- `cache.LRU[K,V]` combines a map with `container/list` to keep a bounded cache in most-to-least-recently-used order.
- `trie.Trie` indexes words by byte and returns prefix matches in ascending byte order, with an optional result limit.
- `topk.TopK` uses `container/heap` to retain the highest counts, breaking ties by the smaller key. Duplicate keys remain separate entries.
- `shortid.CodeGenerator` combines 19 random bits from an injected reader with a 16-bit counter to produce six-character base62 codes. The service still generates its codes from IDs.
- `link.WithCache` accepts the consumer-owned `link.Cache` interface. `Resolve` checks context first, uses cache hits without reading the store, and checks expiry in both cases. Cache misses are filled only after a successful, unexpired store result, so expired links cannot evict valid cached entries. Successful deletion invalidates the cache; `Shorten` and `List` leave it alone. The service also works without a cache.

Tests cover the TCP protocol and shutdown, both commands, the click pipeline, data structures, cache integration, and earlier homework regressions. See [Validation](#validation), [Complexity](#complexity), and [Benchmarks](#benchmarks).

The original interfaces and `Noop*` implementations remain in place. The server loads its database once at startup and does not modify it; restart it to load changes made by `linkctl import`. HTTP, database adapters, and connecting the click pipeline to redirects belong to later milestones.

## Concurrency

- `Counters` starts one owner goroutine; only that goroutine accesses the map of hourly click counts.
- `Add`, `Series`, and `Top` send requests through a channel; queries receive results through buffered reply channels.
- `Pipeline` uses an `RWMutex` to protect its started/closed state and queue closure.
- `Submit` holds an `RLock` while checking the state and attempting to send, allowing concurrent submissions while preventing queue closure; metrics are reported after unlocking.
- Only `Shutdown` closes the queue: it stops new submissions, then waits for workers to drain the queue and flush their final batches.
- If its context expires, `Shutdown` cancels the pipeline's write context; repeated calls wait for the same worker completion.
- Close `Counters` after the pipeline finishes; `Close` signals the owner goroutine and waits for it to exit.
- `LRU` protects its map and list with one `sync.Mutex` in every method, including `Get`, `Len`, and `Keys`.
- `Service.Resolve` may run concurrently when the store, cache, and clock support concurrent access and no caller uses `Shorten` or `Delete`. The daemon only reads the store after startup and uses the synchronized LRU.
- `Store.ByCode` and other reads may run concurrently while no caller uses `Create` or `Delete`; the store itself has no locks.
- The TCP server protects its connection set, closed flag, and read-deadline changes with one mutex. This prevents an idle connection from overwriting the wakeup deadline set by `Shutdown`.
- TCP shutdown closes the listener, wakes idle connections, and waits for active commands to respond. When its deadline expires, it closes remaining connections and cancels their resolver context.

## Environment configuration

`linkforged` and `linkctl config` validate configuration from the environment. Values are trimmed; unset, empty, and whitespace-only variables use the defaults below. Required values have no default. The daemon still requires database credentials and a JWT secret, even though it does not use them to connect to a database or issue tokens yet.

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

The `import`, `export`, `version`, and `resolve` commands do not require database credentials or a JWT secret. They only read `LF_LOG_LEVEL` for logging. `resolve` chooses its server through `-addr`, not `LF_TCP_ADDR`.

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

## Running the resolver locally

Create `links.jsonl` with the import example above, then start the server:

```bash
LF_PG_DSN=unused LF_MONGO_URI=unused \
LF_JWT_SECRET=0123456789abcdef0123456789abcdef \
go run ./cmd/linkforged -db links.jsonl
```

In another terminal, run `go run ./cmd/linkctl resolve 000001`. Use `-addr HOST:PORT` before the code to select another server; the default is `127.0.0.1:7070`. A successful lookup prints only the target URL to stdout and exits 0. An unknown or expired code prints `resolver answered: ERR not_found` to stderr and exits 1; invalid arguments exit 2. The whole client command has a five-second timeout, and cancellation interrupts blocked network I/O.

The daemon writes JSON logs to stderr. Ctrl+C or SIGTERM starts shutdown with a separate 15-second timeout; success logs `resolver stopped` and exits 0.

## TCP resolver and Docker

Requests and replies end in `\n`; `\r\n` is accepted, command names are case-sensitive, and whitespace separates words.

| Request | Response |
|---|---|
| `PING` | `PONG` |
| `RESOLVE <code>` | `OK <target>` or `ERR not_found` |
| `QUIT` | Connection closes without a reply |

Invalid commands return `ERR bad_request` and keep the connection open; unexpected resolver errors return `ERR internal`. Requests exceeding 4,096 bytes including the newline return `ERR line_too_long` and close the connection.

**Docker packaging is pending:** add a multi-stage `Dockerfile` and `.dockerignore` before running this workflow. The required image is under 20 MB, runs as non-root, exposes port 7070, and uses exec-form `ENTRYPOINT ["/linkforged"]`.

```bash
go run ./cmd/linkctl import -in urls.txt -db links.jsonl
docker build -t linkforged .
docker run -d --name linkforged -p 7070:7070 -v "$PWD/links.jsonl:/data/links.jsonl:ro" \
  -e LF_PG_DSN=unused -e LF_MONGO_URI=unused \
  -e LF_JWT_SECRET=0123456789abcdef0123456789abcdef \
  linkforged -db /data/links.jsonl
go run ./cmd/linkctl resolve 000001
docker stop linkforged
docker logs linkforged
docker inspect -f '{{.State.ExitCode}}' linkforged
docker rm linkforged
```

`docker stop` sends SIGTERM to the service. Shutdown stops accepting connections, wakes idle readers, and lets commands already in progress respond before closing their connections. Verify that the container stops well before Docker's default ten-second grace period, logs `resolver stopped`, and exits 0.

## Layout

```
cmd/
  linkctl/                  import, export, config, version, resolve
  linkforged/               TCP service, JSON logs, signal handling

Dockerfile                  planned: multi-stage non-root service image
.dockerignore               planned: exclude Git metadata and local data

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
  transport/tcp/            TCP resolver and graceful shutdown         M6
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

Here, `n` is the number of cached entries, `N` is the number of retained heap items, `m` is the inserted or deleted word's byte length, `L` is the prefix's byte length, and `k` is the number of returned words. For trie traversal, `v` counts visited nodes below the prefix and `D` is the maximum byte length of a returned word. Hashing and comparing cache or heap keys is treated as constant time; longer keys add their hashing or comparison cost.

| Operation | Average / amortized time | Worst-case time |
|---|---|---|
| `LRU.Get` | `O(1)` | `O(n)` |
| `LRU.Put` | `O(1)` | `O(n)` |
| `Trie.Insert` | `O(m)` | `O(m)` |
| `Trie.Delete` | `O(m)` | `O(m)` |
| `Trie.WithPrefix` | `O(L + v + k·D)` | `O(L + v + k·D)` |
| `TopK.Add` | `O(log N)` | `O(log N)` |
| `TopK.Result` | `O(N log N)` | `O(N log N)` |

`LRU.Get` uses the map to find a `*list.Element` in average `O(1)` time. Moving that known element to the front of the doubly linked list changes a fixed number of pointers, so the combined operation remains average `O(1)`. The `O(n)` worst-case bounds account for pathological map collisions and, for insertion, map growth; list operations themselves remain `O(1)`.

`Trie.WithPrefix` first follows the `L` prefix bytes, then visits children in byte order until it has `k` results or exhausts the subtree. The traversal appends to and trims one shared byte buffer, with amortized constant work per edge; each node's 256 child slots are a constant factor. Only complete results are copied into strings, costing at most `O(k·D)` bytes overall. If the prefix itself fills the limit, the method returns before starting the traversal.

`Trie.Delete` records the path in `O(m)` temporary space and removes empty nodes from the leaf upwards, stopping at a stored word or a node with surviving children. Checking 256 child slots per node is a constant factor, so deletion takes `O(m)` time. Pruning releases unused branches for garbage collection and keeps later prefix searches from traversing deleted subtrees.

`TopK.Add` uses a heap push, or a pop followed by a push when replacing the worst retained item. A full heap rejects an item that is not better in `O(1)`; insertion and replacement are bounded by `O(log N)`. `Result` copies the retained items and sorts the copy in `O(N log N)` time, leaving the heap unchanged. Empty and single-item heaps take constant time.

## Benchmarks

Run the benchmarks in the library module from the repository root:

```bash
make bench
```

`BenchmarkLRUGetHit` cycles through 1,024 cached six-character string keys. Every lookup is a hit and moves its entry to the front of the LRU list. `BenchmarkTrieWithPrefix` builds 10,000 words (`alias-0000` through `alias-9999`) and queries `alias-0`, which matches 1,000 words. Its two cases return the first 10 matches or all matches. Both benchmarks use `b.Loop()` and `b.ReportAllocs()`; data preparation is outside the measured loop.

Recorded on 2026-11-15 with Go 1.27.2, macOS/arm64, and an Apple M2:

```text
cd pkg && go test -run '^$' -bench . -benchmem ./...
goos: darwin
goarch: arm64
pkg: github.com/al-bo/rd-linkforge/pkg/cache
cpu: Apple M2
BenchmarkLRUGetHit-8                       68030419         17.58 ns/op         0 B/op         0 allocs/op
PASS
ok      github.com/al-bo/rd-linkforge/pkg/cache       1.459s
PASS
ok      github.com/al-bo/rd-linkforge/pkg/shortid     0.274s
PASS
ok      github.com/al-bo/rd-linkforge/pkg/topk        0.284s
goos: darwin
goarch: arm64
pkg: github.com/al-bo/rd-linkforge/pkg/trie
cpu: Apple M2
BenchmarkTrieWithPrefix/limit_10-8           603356          2548 ns/op       656 B/op        15 allocs/op
BenchmarkTrieWithPrefix/unlimited-8           4353        262839 ns/op     51184 B/op      1011 allocs/op
PASS
ok      github.com/al-bo/rd-linkforge/pkg/trie        3.213s
```

The LRU hit performs no allocations. With a shared traversal buffer, the trie now uses 1,011 allocations for 1,000 matches, down from 1,121 in the previous implementation; limiting the result to 10 words uses 15 allocations. This run took about 263 microseconds for all matches and 2.5 microseconds for 10 matches. Timings depend on the machine and run.

## Validation

Run the Homework 6 checks and earlier regressions from the repository root:

```bash
make check build
gofmt -l .
go test -race -count=5 ./cmd/linkforged ./cmd/linkctl ./internal/transport/tcp
go test -cover ./...
(cd pkg && go test -cover ./...)
```

`make check build` runs vet, lint, tests with the race detector, and builds in both modules. `gofmt -l .` should produce no output. The root module's `go test ./...` and `make cover` do not include the nested `pkg` module, so its coverage must be checked separately.

Homework 6 coverage measured on 2026-11-26 with Go 1.26.1 using `go test -race -coverprofile=coverage.out ./...`:

| Package | Minimum | Measured |
|---|---:|---:|
| `internal/transport/tcp` | 80% | 90.8% |
| `cmd/linkforged` | 70% | 86.6% |
| `cmd/linkctl` | 70% | 91.2% |

The daemon tests cover usage and configuration errors, missing or malformed databases, duplicate codes, occupied ports, credential redaction, and a complete lifecycle with eight concurrent clients. Startup is synchronized through a concurrency-safe JSON log writer; cancellation must return exit code 0, close an idle connection, log `resolver stopped`, and release the port. Repeated lookups exercise the real LRU; the domain cache tests separately verify that hits bypass the store.

The resolver client tests use a real TCP server for successful, missing, expired, and failed lookups. They also check invalid arguments, unavailable servers, response parsing, and interruption of a blocked read by a 100-millisecond parent deadline. The supplied `internal/transport/tcp/hw6_test.go` remains unchanged.

Homework 4 requires at least 85% statement coverage in each new or extended library package. Recorded on 2026-11-15 with `cd pkg && go test -cover ./...`:

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

Library tests cover LRU eviction and recency, a nil cache returned for invalid capacity, trie insertion/deletion, branch pruning and byte ordering, bounded heap ranking and independent result copies, and the generator's reference codes, read errors, and counter wraparound. Cache integration tests cover hits and misses, expiry, context cancellation, store errors, deletion ordering, operation without a cache, and a real LRU passed to `WithCache`. Expired store results are checked to leave the cache unchanged and preserve valid LRU entries.

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
