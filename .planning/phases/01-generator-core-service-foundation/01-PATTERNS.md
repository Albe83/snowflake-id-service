# Phase 1: Generator Core & Service Foundation - Pattern Map

**Mapped:** 2026-07-26  
**Files analyzed:** 10 planned files (greenfield; inferred from CONTEXT.md and RESEARCH.md)  
**Analogs found:** 0 / 10 in-repository; all patterns below are prescribed by the phase research

This repository is greenfield: it currently contains no Go source, module, tests, or runtime configuration to copy. The planner should treat the research artifacts as the authoritative external/reference patterns and establish these conventions for later phases.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `go.mod` | config | transform (module metadata) | None | no analog |
| `internal/idgen/layout.go` | utility/model | transform (bit layout) | None | no analog |
| `internal/idgen/generator.go` | service | request-response; stateful CRUD-like generation | None | no analog |
| `internal/idgen/generator_test.go` | test | batch/concurrency; transform verification | None | no analog |
| `internal/config/config.go` | config | transform (environment → validated config) | None | no analog |
| `internal/config/config_test.go` | test | batch (validation matrix) | None | no analog |
| `cmd/snowflake-service/main.go` | controller/composition root | request-response; lifecycle/event-driven shutdown | None | no analog |
| `README.md` | documentation/config contract | transform (operator/developer contract) | None | no analog |
| `internal/idgen` test fixtures/helpers (if split from `generator_test.go`) | test/utility | transform; frozen-clock injection | None | no analog |
| `cmd/snowflake-service` smoke/lifecycle tests (if split during planning) | test | request-response/event-driven | None | no analog |

The final two rows are permitted test-file splits, not additional product components. Prefer keeping them in the package test files unless the implementation becomes unwieldy. No `pkg/`, HTTP API package, Kubernetes manifests, metrics package, or external runtime dependency belongs to this phase.

## Pattern Assignments

### `go.mod` (config, transform)

**Analog:** None. Initialize a Go 1.22+ module using the repository's eventual import path; keep Phase 1 stdlib-only. Research explicitly calls for `go mod init` in STACK.md and zero external runtime dependencies.

**Pattern to copy:** Pin the `go` directive to the toolchain used by CI and the Docker builder; do not add `prometheus/client_golang` until the observability phase. The module must support `go vet ./...` and `go test -race ./...`.

### `internal/idgen/layout.go` (utility/model, transform)

**Analog:** None in-repo. External reference: `.planning/research/ARCHITECTURE.md` lines 116-145 and `.planning/phases/01-generator-core-service-foundation/01-RESEARCH.md` lines 40-49.

**Bit-layout pattern** (ARCHITECTURE.md lines 119-125):
```go
type Generator struct {
	mu     sync.Mutex
	nodeID int64
	epoch  int64 // custom epoch, ms since Unix epoch
	lastMs int64
	seq    int64
}
```

Define named shifts/masks for 41 timestamp bits, 10 node bits, and 12 sequence bits. Keep the epoch as a hardcoded package constant equal to `1767225600000` (2026-01-01T00:00:00Z), with a comment that the 41-bit range expires around 2095. Do not make the epoch an environment variable. Keep layout helpers pure and free of HTTP/config imports.

### `internal/idgen/generator.go` (service, request-response)

**Analog:** None in-repo. External reference: `.planning/research/ARCHITECTURE.md` lines 104-147; this is the canonical implementation pattern.

**Imports pattern:** stdlib only (`sync`, `time`, and any small error/formatting support required). `idgen` must not import `config`, HTTP, logging, or observability.

**Core mutex pattern** (ARCHITECTURE.md lines 126-145):
```go
func (g *Generator) Generate() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.clock().UnixMilli() - g.epoch
	if now < g.lastMs {
		return 0, ErrClockMovedBackwards
	}
	if now == g.lastMs {
		g.seq = (g.seq + 1) & seqMask
		if g.seq == 0 {
			for now <= g.lastMs {
				now = g.clock().UnixMilli() - g.epoch
			}
		}
	} else {
		g.seq = 0
	}
	g.lastMs = now
	return now<<timeShift | g.nodeID<<nodeShift | g.seq, nil
}
```

Adapt the reference shape to an injectable clock so tests can freeze and move time backwards. The clock-skew check must occur while holding the mutex; return a typed/sentinel `ErrClockMovedBackwards`, return ID `0`, and do not mutate `lastMs` or `seq` on rejection. Sequence exhaustion spins to the next millisecond rather than returning an overflow error. Constructor validation must reject node IDs outside 0–1023 and a clock before the epoch; the constructor should return an error rather than hiding invalid state.

Implement `Decode(id)` as a stateless pure bit-shift operation returning timestamp, node ID, and sequence. Keep generated values positive signed `int64`; no package-level mutable generator or default singleton.

### `internal/idgen/generator_test.go` (test, batch/concurrency and transform)

**Analog:** None. External reference: `.planning/phases/01-generator-core-service-foundation/01-RESEARCH.md` lines 60-68 and `.planning/research/PITFALLS.md` lines 94-131.

**Testing pattern:** table-driven constructor/validation tests, deterministic injected-clock tests, and a real concurrent uniqueness test. Required cases:

- generated ID is positive;
- `Decode(Generate())` round-trips timestamp, node ID, and sequence;
- clock moves backwards → typed error, ID 0, unchanged state, and a later call re-checks the clock;
- frozen clock generates 4096 IDs, then the 4097th advances to the next millisecond (use a controllable clock that lets the test release the spin);
- N goroutines × M IDs, collect all IDs in a synchronized result channel/map, assert global uniqueness;
- run this package under `go test -race`; do not rely only on single-threaded tests.

Tests should use constructors and injected clocks, not package globals or sleeps as the primary synchronization mechanism.

### `internal/config/config.go` (config, transform)

**Analog:** None in-repo. External reference: `.planning/research/ARCHITECTURE.md` lines 53-62 and 214-225; `.planning/phases/01-generator-core-service-foundation/01-RESEARCH.md` lines 51-58.

**Environment loading pattern:** read `NODE_ID` and `PORT` once with `os.Getenv`, parse with `strconv`, and return one validated immutable `Config` struct. `NODE_ID` is mandatory, always explicit, numeric, and constrained to 0–1023. `PORT` has a documented sane default (research suggests 8080) and must be validated as a usable listener port. There is no `POD_NAME` ordinal parsing and no `EPOCH` environment variable.

**Error pattern:** return descriptive errors from `Load`/validation; let the composition root log them and terminate non-zero. Config must also expose or support the startup check that the current clock is at or after the hardcoded idgen epoch. Nothing re-reads environment variables after boot.

### `internal/config/config_test.go` (test, batch)

**Analog:** None. External reference: `.planning/phases/01-generator-core-service-foundation/01-RESEARCH.md` lines 64-67.

Use table-driven environment cases with an injected environment source or isolated `t.Setenv`: missing `NODE_ID`, non-numeric value, negative/out-of-range values, invalid port, default port, and valid values. Cover the pre-epoch startup validation with a controllable clock. Assert errors rather than process exit inside package tests; exit behavior is owned by `main`.

### `cmd/snowflake-service/main.go` (composition root/controller, request-response and event-driven)

**Analog:** None in-repo. External reference: `.planning/research/ARCHITECTURE.md` lines 53-60, 214-225, and 182-186.

**Composition pattern:** keep `main()` thin (ideally call `run() error`): load/validate config, create the generator, create a minimal stdlib `http.ServeMux`/server, log startup identity, and own lifecycle. No generation logic, mutable package globals, or environment parsing should live here beyond wiring.

**Logging pattern:** configure `log/slog` with a JSON handler. The successful boot log must include `node_id` and the epoch; startup/config failures go to the logger and produce a non-zero exit.

**Shutdown pattern** (ARCHITECTURE.md lines 182-185):
```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()

go func() {
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		// report server failure to the run/lifecycle path
	}
}()

<-ctx.Done()
shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
defer cancel()
return srv.Shutdown(shutdownCtx)
```

Register exactly one temporary `GET /dev/id` route under `/dev/`, with a code comment that it is replaced by the Phase 2 API. Use a narrow generator dependency where practical. Encode the response with `encoding/json` as `{"id":"<decimal string>"}`—never as a JSON number. The route should map generator errors to a minimal server error response; full API status/error conventions are deferred to Phase 2.

### `README.md` (documentation/config contract, transform)

**Analog:** None. External reference: CONTEXT.md D-05–D-08, D-10–D-13 and `.planning/research/PITFALLS.md` lines 73-90.

Document local startup (`NODE_ID=0`, optional `PORT`, `go run ./cmd/snowflake-service`), the explicit-node-ID contract, the temporary `/dev/id` smoke route, and that IDs are JSON strings. Record the fixed epoch and “IDs valid until ~2095” warning, the 0–1023 node range, and that a boot failure for invalid config or a pre-epoch clock is intentional fail-fast behavior. Do not document the superseded StatefulSet/ordinal behavior as a Phase 1 contract.

## Shared Patterns

### Dependency direction and construction

**Source:** `.planning/research/ARCHITECTURE.md` lines 95-100 and 332-341.  
**Apply to:** all Go files.

`cmd` constructs dependencies; `config` produces plain data; `idgen` remains a pure stdlib kernel; transport calls the generator rather than the reverse. No package-level mutable state, no `pkg/`, and no cross-package import from `idgen` into config or HTTP.

### Fail-fast startup

**Source:** `.planning/research/ARCHITECTURE.md` lines 214-225.  
**Apply to:** `internal/config/config.go`, `cmd/snowflake-service/main.go`, `README.md`.

The order is config load → node/port/epoch validation → generator construction → mux/server construction → serve. Invalid configuration logs and exits non-zero before the listener starts. The boot log proves `node_id` and epoch.

### String IDs at JSON boundaries

**Source:** `.planning/research/ARCHITECTURE.md` lines 194-198 and `.planning/research/PITFALLS.md` lines 54-69.  
**Apply to:** `cmd/snowflake-service/main.go`, future HTTP handlers.

Internally use signed `int64`; at the HTTP/JSON boundary format IDs as quoted decimal strings to avoid JavaScript 2^53 precision loss.

### Correctness gates

**Source:** `.planning/research/STACK.md` lines 25-32, `.planning/research/PITFALLS.md` lines 170-176.  
**Apply to:** all implementation and test files.

The phase gate is `go vet ./... && go test -race ./...`; the frozen-clock 4097-ID test, backwards-clock rejection test, positivity test, decode round-trip, and quoted-ID smoke response are mandatory—not optional examples.

## No Analog Found

Every Phase 1 file has no close in-repository analog. The repository contains only `AGENTS.md` and `LICENSE`; there is no Go module, source tree, test convention, server, config loader, or README implementation to copy. Use the cited research excerpts as reference patterns, while respecting the locked amendments in CONTEXT.md: explicit `NODE_ID` only, hardcoded epoch, no metrics, no Kubernetes manifests, and only the temporary dev route.

## Metadata

**Analog search scope:** repository root, `cmd/**`, `internal/**`, `**/*.go`, `go.mod`, and project documentation.  
**Files scanned:** 2 existing repository files plus the four phase/reference artifacts.  
**Pattern extraction date:** 2026-07-26
