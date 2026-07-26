# Architecture Research

**Domain:** Distributed unique ID generation service (Snowflake-style 41/10/12) — small Go REST microservice, multi-instance on Kubernetes, no coordination between instances
**Researched:** 2026-07-26
**Confidence:** HIGH overall — component patterns verified against primary sources (bwmarrin/snowflake source, go.dev docs, kubernetes.io docs) on 2026-07-26. Note: the gsd-tools confidence seam rates blind `webfetch` as LOW by default; all findings below were upgraded per the verification protocol because every load-bearing claim was checked directly against official/primary material, and the key ones were cross-checked across two independent sources.

## Standard Architecture

The domain-standard shape is a **single-binary, stateless-per-process Go service** with one stateful kernel (the generator) guarded by a mutex. Every instance is self-sufficient: uniqueness across instances comes entirely from distinct static node IDs baked into config, never from runtime coordination.

### System Overview (one instance)

```
┌────────────────────────────────────────────────────────────────┐
│                    cmd/snowflake-service (main)                 │
│        composition root: build config → generator → server      │
│        own lifecycle: signal.NotifyContext → srv.Shutdown()     │
├────────────────────────────────────────────────────────────────┤
│                     internal/httpapi                            │
│  ┌──────────────┐ ┌──────────────┐ ┌────────────────────────┐  │
│  │ POST /v1/ids │ │ health probe │ │ ops: /metrics, decode  │  │
│  │  handler     │ │  handlers    │ │  handlers              │  │
│  └──────┬───────┘ └──────────────┘ └────────────────────────┘  │
│         │ calls Generator interface (narrow: Generate() (int64,│
│         │ error), Decode(id) Parts)                            │
├─────────┼──────────────────────────────────────────────────────┤
│  internal/idgen (the only stateful component)                   │
│  ┌────────▼───────────────────────────────────────────────┐    │
│  │ Generator: mu sync.Mutex { lastMs, seq } + nodeID,     │    │
│  │ epoch, clock-skew policy seam. Pure compute, no I/O.   │    │
│  └────────────────────────────────────────────────────────┘    │
├────────────────────────────────────────────────────────────────┤
│ internal/config (env → validated struct, fail-fast at startup) │
│ internal/observability (slog logger, prometheus registry)      │
└────────────────────────────────────────────────────────────────┘
```

### Fleet View (Kubernetes)

```
                 ┌──────────────────────────┐
                 │   Service (ClusterIP)    │  load-balances across pods
                 └───────┬──────────┬───────┘
            ┌────────────┤          ├────────────┐
     ┌──────▼─────┐ ┌────▼──────┐ ┌─▼──────────┐
     │ pod sts-0  │ │ pod sts-1 │ │ pod sts-2  │   StatefulSet: stable
     │ NODE_ID=0  │ │ NODE_ID=1 │ │ NODE_ID=2  │   ordinal per pod
     └────────────┘ └───────────┘ └────────────┘
     No pod-to-pod traffic. No shared state. No leader. No leases.
     Prometheus scrapes /metrics on each pod directly.
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| `cmd/<name>/main.go` | Composition root only: parse config, wire dependencies, start server, handle shutdown signals | Thin `main()` calling `run() error`; no business logic |
| `internal/idgen` | The 41/10/12 bit layout, sequence management, millisecond timing, clock-skew policy seam, ID decode | One `Generator` struct with `sync.Mutex`; pure compute, trivially unit-testable |
| `internal/httpapi` | HTTP transport: routing, JSON encode/decode, request validation, status codes, HTTP-level middleware (logging, metrics) | stdlib `net/http` + Go 1.22 `ServeMux` patterns; handlers depend on a narrow `Generator` interface, not the concrete type |
| `internal/config` | Load env vars, resolve node ID (explicit `NODE_ID` or derived from pod ordinal), validate ranges, fail fast | `os.Getenv` + `strconv` + one `Config` struct; no config library needed at this size |
| `internal/observability` | Structured logging, Prometheus metric definitions + registry, health state | `log/slog` (stdlib), `prometheus/client_golang` + `promhttp` |
| Kubernetes manifests | Stable per-pod identity (node ID), probes, disruption budget, rollout strategy | StatefulSet + Service + PDB; downward API injects pod name |

## Recommended Project Structure

```
snowflake-id-service/
├── cmd/
│   └── snowflake-service/
│       └── main.go            # composition root + lifecycle only
├── internal/
│   ├── idgen/
│   │   ├── generator.go       # Generator struct, Generate(), Decode()
│   │   ├── generator_test.go  # incl. concurrency + clock-skew tests
│   │   └── layout.go          # bit constants: shifts, masks, epoch
│   ├── httpapi/
│   │   ├── server.go          # mux construction, middleware chain
│   │   ├── ids_handler.go     # POST /v1/ids (single + batch)
│   │   ├── ops_handler.go     # /healthz /readyz /metrics wiring
│   │   └── decode_handler.go  # GET /v1/ids/{id} (added later)
│   ├── config/
│   │   └── config.go          # env loading, node-ID resolution, validation
│   └── observability/
│       ├── logging.go         # slog setup
│       └── metrics.go         # counters/gauges + promhttp handler
├── deployments/               # or deploy/
│   ├── statefulset.yaml
│   ├── service.yaml
│   └── pdb.yaml
├── Dockerfile                 # multi-stage: golang builder → scratch/distroless
├── go.mod
└── Makefile                   # optional convenience
```

### Structure Rationale

- **`cmd/` + `internal/` is the whole skeleton.** This matches both the official Go module-layout guidance and the de-facto community layout (golang-standards/project-layout), whose own README warns the full layout is **overkill for small projects** and explicitly notes it is *not* an official standard. For a single-binary microservice, `cmd/` + `internal/` + `deployments/` covers everything.
- **No `pkg/` for now.** `pkg/` declares "safe for external import". Nothing here needs importing by other modules today — Foundation projects will call the service over REST. If in-process ID generation ever becomes a requirement, promote `internal/idgen` → `pkg/idgen` then; starting in `internal/` keeps that decision open while the compiler enforces the boundary. (Verified: the `internal/` import restriction is compiler-enforced per Go 1.4 release notes, referenced in the layout doc.)
- **Package boundaries = dependency direction.** `httpapi` → `idgen` ← `config`; nothing in `idgen` imports HTTP, config, or observability packages. The generator stays a pure, deterministic kernel — this is what makes the core-value property ("no duplicate IDs") testable in isolation.
- **`deployments/` for K8s YAML** follows the community convention; manifests are part of the architecture here because the node-ID mechanism lives in them.

## Architectural Patterns

### Pattern 1: Mutex-Guarded Stateful Generator (the canonical design)

**What:** A single `Generator` struct per process holds the mutable generation state (`lastMillisecond`, `sequence`) behind a `sync.Mutex`. `Generate()` locks, reads the clock, increments the sequence if still in the same millisecond, and composes the 64-bit ID with bit shifts.
**When to use:** Always, for this design. Verified as the reference implementation's approach (bwmarrin/snowflake, 3.3k stars, source read 2026-07-26: `Node.mu sync.Mutex` around the whole `Generate()`).
**Trade-offs:** A mutex serializes concurrent HTTP handler goroutines — irrelevant here (expected volume is low; the reference achieves ~244 ns/op, i.e. millions of IDs/sec/node under contention-free conditions). The alternatives are strictly worse *for this service*:

| Model | Verdict | Why |
|-------|---------|-----|
| **Mutex** (recommended) | ✅ | Simplest correct code; standard pattern; contention cost invisible at low volume |
| Atomic CAS loop | ❌ | Lock-free packing of (time, seq) into one `atomic.Int64` works but is ~2-3x the code, easier to get wrong on overflow/skew paths, and buys throughput this service will never need |
| Channel-owned generator goroutine | ❌ | "Share memory by communicating" applied dogmatically: adds a goroutine + channel hop per ID, complicates error propagation and shutdown, still caps at 4096/ms |

**Example (verified design shape):**
```go
type Generator struct {
    mu     sync.Mutex
    nodeID int64
    epoch  int64 // custom epoch, ms since Unix epoch
    lastMs int64
    seq    int64
}

func (g *Generator) Generate() (int64, error) {
    g.mu.Lock()
    defer g.mu.Unlock()

    now := time.Now().UnixMilli() - g.epoch
    if now < g.lastMs {
        return 0, ErrClockMovedBackwards // ← the deferred skew decision lives HERE
    }
    if now == g.lastMs {
        g.seq = (g.seq + 1) & seqMask // 4095
        if g.seq == 0 {               // sequence exhausted this millisecond
            for now <= g.lastMs {     // spin-wait for next ms (standard behavior)
                now = time.Now().UnixMilli() - g.epoch
            }
        }
    } else {
        g.seq = 0
    }
    g.lastMs = now
    return now<<timeShift | g.nodeID<<nodeShift | g.seq, nil
}
```

### Pattern 2: Spin-Wait on Sequence Exhaustion (not error)

**What:** When 4096 IDs are consumed within one millisecond, `Generate()` busy-spins until the clock ticks to the next millisecond, then continues with `seq = 0`.
**When to use:** Default behavior, as verified in the reference implementation. At this project's stated low volumes it will effectively never trigger; the spin is bounded (~≤1 ms) and keeps the API contract simple (no overflow error type for callers to handle).
**Trade-offs:** The alternative — returning a `503`/error on overflow — pushes a normal-condition retry onto clients for zero benefit at low volume. Keep the spin; surface it as a **metric** (`snowflake_sequence_waits_total`) so future scale surprises are visible.

### Pattern 3: Clock-Skew Policy as a Seam in the Core (decision deferred, boundary fixed now)

**What:** The generator — and only the generator — detects `now < lastMs` and owns the response. `Generate()` returns `(int64, error)` so both candidate strategies fit without changing component boundaries later:
- **Reject-with-error:** return `ErrClockMovedBackwards`; handler maps to `503 Service Unavailable`. Simple, honest, alerts fire.
- **Wait:** block (with a bounded timeout) until wall clock catches up; favors availability over latency predictability.

**Verified design input for that planning discussion:** the reference implementation sidesteps the choice entirely by anchoring to Go's **monotonic clock** (`n.epoch = curTime.Add(...Sub(curTime))` + `time.Since(n.epoch)`), making NTP steps *invisible* within a process run. That works but means embedded ID timestamps silently drift from wall-clock time, and it provides zero skew *detection* — this project explicitly wants skew **handled**, so the wall-clock + explicit-policy seam above is the right architecture; the monotonic option can still be evaluated as strategy #3 in the planning discussion.
**Also fixed at architecture time:** validate `time.Now() >= customEpoch` at startup and fail fast — a clock reading before the epoch shifts the sign bit and produces negative IDs (verified gap in the reference library, which performs no such check).

### Pattern 4: Narrow Interface at the HTTP Boundary

**What:** Handlers never see the concrete generator. `httpapi` defines its own minimal interface:
```go
type Generator interface {
    Generate() (int64, error)
    Decode(id int64) Parts // Parts{Timestamp, NodeID, Sequence}
}
```
**When to use:** Always at package seams in Go ("accept interfaces, return structs").
**Trade-offs:** None at this size. Buys: handler tests with a fake generator (deterministic skew/overflow scenarios), and freedom to reimplement the core without touching transport code.

### Pattern 5: Stdlib-Only HTTP with Go 1.22 ServeMux Patterns

**What:** Register routes with method-aware patterns — `mux.HandleFunc("POST /v1/ids", h)`, `mux.HandleFunc("GET /v1/ids/{id}", h)` — and read path values via `r.PathValue("id")`. Method mismatches get automatic `405` + `Allow` header; conflicting patterns panic at startup (fail-fast).
**When to use:** For this service's ~5 routes. Verified in the Go 1.22 routing enhancements (go.dev blog, Feb 2024).
**Trade-offs:** No router dependency (chi/gorilla) to vendor, audit, or upgrade. If route count ever grows past ~15 or needs regex matching, chi drops in without architectural change (it's still just `http.Handler`).

### Pattern 6: Graceful Shutdown via signal.NotifyContext + Server.Shutdown

**What:** `ctx, stop := signal.NotifyContext(context.Background(), SIGINT, SIGTERM)` in main; run `srv.ListenAndServe()` in a goroutine; on `<-ctx.Done()` call `srv.Shutdown(timeoutCtx)` which stops accepting and drains in-flight requests. Verified stdlib API (`signal.NotifyContext` since Go 1.16; `Server.Shutdown` since Go 1.8).
**When to use:** Always on K8s — pods receive SIGTERM on rollout/eviction, and this project's goal is *near-zero downtime maintenance*.
**Trade-offs:** Pair with K8s mechanics: `terminationGracePeriodSeconds` (default 30s) must exceed the shutdown timeout; set readiness to fail as soon as shutdown begins so the endpoints controller stops routing new requests to the draining pod (readiness flip + endpoint propagation has a known small race — a short pre-stop delay or simply the drain window covers it at this traffic level).

### Pattern 7: Static Node Identity via StatefulSet Ordinal + Downward API

**What:** Deploy as a StatefulSet; inject the pod name through the downward API (`env: POD_NAME` ← `fieldRef: metadata.name`); the config package parses the trailing ordinal (`snowflake-7` → node ID 7). Explicit `NODE_ID` env var overrides for local dev/single-binary runs.
**When to use:** This is **the** answer to "static config, no coordination, N replicas, distinct node IDs" on Kubernetes. StatefulSets guarantee: ordinals `0..N-1` unique across the set, stable identity across reschedules (a replacement pod keeps its ordinal → keeps its node ID), ordered rolling updates. All documented, stable K8s behavior (verified against kubernetes.io StatefulSet docs).
**Trade-offs:** See the Deployment-model comparison under Scaling below. Requires app code to parse the ordinal (5 lines) and manifests to wire the downward API. Cap replicas ≤ 1024 (10 node bits) — enforce in config validation and, optionally, an admission check.

### Pattern 8: IDs as Strings in JSON

**What:** Serialize generated IDs as JSON **strings** (`{"id": "7234567890123456789"}`), accept strings on the decode endpoint.
**When to use:** Always, for 63-bit values crossing JSON boundaries.
**Trade-offs:** JavaScript's `Number.MAX_SAFE_INTEGER` is 2^53-1 — raw numeric snowflake IDs silently lose precision in any JS/web consumer. Verified mitigation: the reference Go library's own `MarshalJSON` emits quoted strings. Slight ergonomic cost for Go-to-Go consumers, who must `ParseInt` — acceptable; document it in the API contract.

### Pattern 9: Thin Health Endpoints, Honest Readiness

**What:** `/healthz` (liveness) = "process is up" — no dependency checks (there are none anyway). `/readyz` (readiness) = "generator initialized with valid node ID" + flips to failing during shutdown drain.
**When to use:** Always on K8s. Liveness failing → kubelet restarts the pod (pointless and harmful for anything but deadlock); readiness failing → pod leaves Service endpoints (the correct signal for "don't route here now").
**Trade-offs:** None. Two tiny handlers; both registered on the same mux/port for MVP simplicity.

### Pattern 10: Prometheus Metrics via promhttp, Node-Labeled

**What:** Mount `promhttp.Handler()` at `/metrics`; define `ids_generated_total{node_id}` counter, `snowflake_clock_skew_events_total`, `snowflake_sequence_waits_total`, plus request duration instrumentation via `promhttp.InstrumentHandlerDuration`.
**When to use:** From the ops-endpoints phase onward. Verified against `prometheus/client_golang` v1.24.1 docs — `promhttp.Handler()` covers the standard case with zero options; `node_id` label cardinality is ≤1024, safe.
**Trade-offs:** The only external runtime dependency besides the stdlib. Justified: Prometheus is the K8s-ecosystem metrics standard, and per-node counters are exactly what you need to *prove* the no-duplicates property in production (per-node rate visibility makes collisions detectable).

## Data Flow

### Startup Flow (fail-fast ordering matters)

```
process start
  → config.Load(): read env (PORT, NODE_ID | POD_NAME, EPOCH override, timeouts)
  → validate: 0 ≤ nodeID ≤ 1023; time.Now() ≥ epoch; port valid
      └─ invalid → log + exit(1)  [CrashLoop surfaces misconfig immediately]
  → observability: slog logger + prometheus registry
  → idgen.New(nodeID, epoch, skewPolicy)
  → httpapi.NewServer(gen, logger, registry) — build mux
  → srv.ListenAndServe() in goroutine; main blocks on signal ctx
  → SIGTERM/SIGINT → readyz starts failing → srv.Shutdown(drain) → exit 0
```

### Request Flow — POST /v1/ids (generate, incl. batch)

```
client
  → K8s Service (kube-proxy) → one pod :PORT
  → middleware: request logging (slog), metrics timer
  → handler: parse + validate {count: 1..1000} (cap batch to bound lock hold time)
  → loop count × Generator.Generate()
        ├─ (mutex) read wall clock → same ms? seq++ : reset seq
        ├─ skew? → error path (per deferred policy) → 503 + error body
        └─ seq overflow? → spin to next ms (standard)
  → marshal {"ids": ["<string>", ...]}  ← strings, not numbers
  → 200 OK
```

### Request Flow — GET /v1/ids/{id} (decode, later phase)

```
client → pod → mux ("GET /v1/ids/{id}") → r.PathValue("id")
  → strconv.ParseInt → Generator.Decode(id)  (pure bit-shift, stateless, no lock)
  → {"timestamp": <ms since epoch>, "node_id": N, "sequence": M, "iso8601": "..."}
```

### Key Data Flows

1. **Configuration (one-directional, startup only):** env vars / downward API → `config.Config` (immutable struct) → constructor args. Nothing re-reads env after boot; nothing writes back. The pod name is the only "dynamic" input, and it is static for the pod's lifetime.
2. **Generation (the hot path):** handler → generator mutex → response. No I/O, no allocation beyond the response slice, no inter-pod communication. Sub-millisecond end-to-end.
3. **Metrics (out-of-band):** handlers/generator increment counters in the registry; Prometheus scrapes `/metrics` per pod independently. Metrics never flow through the request path synchronously.
4. **Health (control-plane only):** kubelet → `/healthz`; endpoints controller observes `/readyz`. Neither touches generator state.
5. **Decode (pure):** any pod can decode any ID — decode is a pure function of the bit layout, not of local state. No routing affinity needed.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| 1 instance (dev) | Single binary, explicit `NODE_ID=0`. Skip K8s entirely. |
| 2–5 replicas (target prod) | StatefulSet + downward-API ordinal node IDs + Service + PDB (`maxUnavailable: 1`). This is the design point — resilience, not load. |
| 10–1024 replicas | Same architecture, unchanged. Watch: node-ID bookkeeping (which ordinals are/were used) becomes an ops concern; per-node metrics labels grow linearly. |
| >1024 replicas or >4096 ID/ms/node | **Bit-layout exhaustion, not architecture.** Requires layout change (fewer node bits, more sequence bits, or wider timestamps) — a new service version, not a config change. |

### Scaling Priorities

1. **First "bottleneck" is not throughput — it's identity bookkeeping.** The mutex generator does ~4M IDs/s/node; the binding constraint is the 1024-node ID space and knowing which node IDs are (or were ever) live, because a reused node ID + clock skew is the duplicate-ID vector. Keep a human-managed registry (even a markdown table) of assigned node IDs.
2. **Second: clock hygiene.** All uniqueness arguments assume wall clocks roughly synchronized across nodes. On K8s, pod clocks are the *node* kernel's clock — ensure cluster nodes run NTP/chrony (cluster-admin concern), alert on `snowflake_clock_skew_events_total`.

### Deployment-Model Comparison for Static Node IDs (decision input)

| Model | Node-ID mechanism | Verdict |
|-------|-------------------|---------|
| **StatefulSet + downward API** (recommended) | Ordinal parsed from `metadata.name` | ✅ Zero coordination, self-healing keeps identity, native K8s, scales via `replicas` |
| Deployment, one env var for all pods | Impossible — every replica gets the **same** NODE_ID → guaranteed duplicates | ❌ Architecturally invalid for this service |
| N separate Deployments (1 replica each, distinct env) | Manual per-Deployment `NODE_ID` | ⚠️ Works but: N manifests to maintain, scaling = adding manifests, identity lost on pod replacement unless pinned — operational toil for zero gain |
| Deployment + init-container lease (etcd/Redis) | Runtime-coordinated ID | ❌ Explicitly out of scope (violates the no-coordination constraint) |

**Implication for the roadmap:** the StatefulSet + downward-API pattern is the only model that satisfies all stated constraints (static config, no coordination, self-healing, multi-instance). It should be the default manifest; single-binary/dev mode keeps explicit `NODE_ID`.

## Anti-Patterns

### Anti-Pattern 1: Cargo-Culting the Full golang-standards Layout

**What people do:** Create `pkg/`, `api/`, `web/`, `configs/`, `scripts/`, `build/`, `third_party/` directories on day one because the famous repo lists them.
**Why it's wrong:** The repo's own README says it's overkill for small projects and is *not* an official standard. Empty directories obscure the real structure and invite misplaced code.
**Do this instead:** `cmd/` + `internal/` + `deployments/` only; add directories when a real file needs them.

### Anti-Pattern 2: Global Mutable Generator / Package-Level Config

**What people do:** `var DefaultGenerator = ...` or package-level `var Epoch` mutated in `init()` (the reference library does this — and has marked those globals **DEPRECATED**).
**Why it's wrong:** Hidden coupling, untestable without global state surgery, races on reconfiguration, and two tests with different epochs poison each other.
**Do this instead:** Everything constructed in `main()` and passed down (`NewGenerator(cfg)`, `NewServer(gen, ...)`). One composition root.

### Anti-Pattern 3: Atomic/CAS or Channel-Based Generator for "Performance"

**What people do:** Rewrite the generator lock-free (CAS loop on packed time+seq) or behind a channel-serving goroutine before measuring.
**Why it's wrong:** 4096 IDs/ms/node is the format ceiling regardless of synchronization strategy; the mutex version already saturates it at ~244 ns/op. The alternatives multiply complexity on the two hardest paths (overflow spin, skew handling) for zero user-visible benefit.
**Do this instead:** Mutex. Optimize only if profiling (that this service will never need) says otherwise.

### Anti-Pattern 4: Framework/Router Dependency for 5 Routes

**What people do:** Pull in gin/echo/chi + transitive dependencies for a service with four endpoints.
**Why it's wrong:** Go 1.22's stdlib mux has method patterns, wildcards, and 405 handling. A framework adds supply-chain surface, upgrade churn, and hides the (tiny) HTTP layer behind framework idioms.
**Do this instead:** stdlib `net/http` + `prometheus/client_golang` as the *only* external dependency. Adopt chi later only if routing genuinely outgrows the mux.

### Anti-Pattern 5: Emitting Raw int64 IDs in JSON

**What people do:** `json.Marshal` the `int64` directly because Go handles it fine.
**Why it's wrong:** Any JavaScript/web consumer (and several JSON tooling chains) parses numbers as IEEE-754 doubles — precision silently breaks above 2^53. Duplicate-detection bugs downstream, caused here.
**Do this instead:** String-encode IDs at the API boundary (verified: this is what the reference library's own `MarshalJSON` does). Internally, everything stays `int64`.

### Anti-Pattern 6: Deployment with Shared NODE_ID

**What people do:** `kubectl apply` a Deployment with `replicas: 3` and one `NODE_ID: 1` env var, because "it's stateless anyway."
**Why it's wrong:** All three pods now generate from the same (node, ms, seq) space → **guaranteed duplicate IDs**, the exact failure the service exists to prevent. This is the single most dangerous misconfiguration for this architecture.
**Do this instead:** StatefulSet + ordinal (Pattern 7), plus a boot-time log line of the resolved node ID, plus per-node metrics so collisions are *visible* if they ever happen.

## Integration Points

### External Services / Platform

| System | Integration Pattern | Notes |
|--------|--------------------|-------|
| Kubernetes | StatefulSet, Service (ClusterIP), PDB, probes (`/healthz`, `/readyz`), downward API (`metadata.name` → env) | `terminationGracePeriodSeconds` ≥ shutdown timeout; readiness flip on shutdown start |
| Prometheus | Scrape `/metrics` per pod | `prometheus.io/scrape` annotations or ServiceMonitor; per-node-ID label cardinality ≤1024 is fine |
| Client applications (Foundation services) | REST `POST /v1/ids` | Contract: IDs are JSON strings; batch `count` capped; 503 signals clock-skew policy rejection |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `main` → all packages | Constructor calls only | One-directional wiring; no package imports `cmd` |
| `httpapi` → `idgen` | Narrow `Generator` interface (defined in `httpapi`) | Fake-able for handler tests |
| `httpapi` → `observability` | Direct: logger handle, registry handle passed to `NewServer` | Middleware pattern |
| `config` → `idgen` | Plain data (`Config` struct fields as constructor args) | `idgen` never imports `config` |
| `idgen` → nothing | stdlib `time`, `sync` only | The purity that makes the core provable |
| K8s manifests → `config` | Env var names + ordinal parsing contract | Cross-artifact contract — rename env vars in both places together |

## Suggested Build Order (for roadmap phases)

Ordered by dependency, each phase independently deployable:

1. **`internal/idgen` core + tests** — no dependencies, highest-risk component (uniqueness = core value). Include: bit layout, mutex generation, sequence-overflow spin, clock-skew seam with the policy decision taken in planning, startup epoch validation, concurrency test (parallel goroutines → assert no dupes), decode function. *The clock-skew strategy choice blocks only this phase's API shape (error vs wait), not the overall architecture.*
2. **`internal/config` + `cmd` composition root + graceful shutdown** — env loading, validation, `NODE_ID`/`POD_NAME` resolution, `NotifyContext`+`Shutdown` skeleton. Runnable locally generating IDs via a temporary route or REPL-style main.
3. **`internal/httpapi`: POST /v1/ids (+ string-JSON contract) + health endpoints** — first real API slice; health endpoints come here because K8s manifests (next step) reference them.
4. **Container + K8s manifests (StatefulSet, Service, PDB)** — multi-instance deploy; *validates the ordinal→node-ID mechanism end-to-end early*, since it's the riskiest ops assumption. Include an e2e duplicate-check (hammer N pods, assert set uniqueness).
5. **Observability: /metrics + per-node counters + request instrumentation** — proves the no-duplicates property in production; deliberately after multi-instance exists so per-node labels have meaning.
6. **Decode endpoint** — pure function already tested in phase 1; this phase is only the route + handler. Strictly independent; slot whenever convenient.

**Ordering rationale:** the generator's correctness gates everything (core value); config+shutdown gates running anywhere real; the generation endpoint is the MVP per PROJECT.md; K8s multi-instance precedes metrics because multi-instance is the stated resilience goal and the ordinal mechanism deserves the earliest possible end-to-end proof; decode is last because it's the declared nice-to-have with zero architectural risk.

## Sources

| Source | Used for | Fetched |
|--------|----------|---------|
| bwmarrin/snowflake README + `snowflake.go` source (master) | Generator struct/concurrency model, spin-wait overflow, monotonic-clock anchoring, JSON string marshaling, node validation, "stable/completed" status | 2026-07-26 |
| golang-standards/project-layout README | cmd/internal/pkg conventions, "not an official standard" + "overkill for small projects" caveats, internal compiler enforcement (Go 1.4) | 2026-07-26 |
| kubernetes.io — StatefulSets concept page | Stable ordinals, stable network identity, pod naming, guarantees across rescheduling | 2026-07-26 |
| go.dev blog — "Routing Enhancements for Go 1.22" (Feb 2024) | Method patterns, wildcards, `PathValue`, conflict panics, 405 handling | 2026-07-26 |
| pkg.go.dev — `os/signal` (go1.26.5) | `NotifyContext` semantics (added Go 1.16) | 2026-07-26 |
| pkg.go.dev — `promhttp` (client_golang v1.24.1) | `Handler()`, `InstrumentHandlerDuration`, zero-value `HandlerOpts` | 2026-07-26 |

*All primary sources. Cross-checks: concurrency model + overflow behavior (README prose vs. source code), JSON string encoding (library practice vs. JS `Number.MAX_SAFE_INTEGER` semantics). No community/blog sources were needed for load-bearing claims.*

---
*Architecture research for: Snowflake-style distributed ID generation service in Go on Kubernetes*
*Researched: 2026-07-26*

