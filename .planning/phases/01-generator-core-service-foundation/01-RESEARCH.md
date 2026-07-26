# Phase 1: Generator Core & Service Foundation - Research

**Researched:** 2026-07-26
**Domain:** Snowflake-style 41/10/12 distributed ID generation — Go kernel + service bootstrap
**Confidence:** HIGH — all load-bearing claims trace to primary-source-verified research (ARCHITECTURE.md, verified 2026-07-26 against bwmarrin/snowflake source, go.dev, kubernetes.io) and locked CONTEXT.md decisions (D-01..D-16).

## Summary

Phase 1 delivers the uniqueness-critical generator kernel — a mutex-guarded 41/10/12 Snowflake generator with a hardcoded custom epoch (2026-01-01T00:00:00Z) — provably correct in isolation via unit, race-detector, and frozen-clock tests, plus the service foundation: fail-fast static config (`NODE_ID`, `PORT`), a thin composition root (`cmd/snowflake-service/main.go`) with a `signal.NotifyContext` shutdown skeleton, and ONE temporary dev route (`GET /dev/id`) returning JSON string IDs for curl smoke tests (CONTEXT.md D-09, D-10). The real REST API is Phase 2; K8s is Phase 3; metrics are v2 (CONTEXT.md Phase Boundary).

The phase is stdlib-only: **zero external runtime dependencies** (CONTEXT.md carried-forward decisions). Package layout is `cmd/` + `internal/` only — `internal/idgen`, `internal/config`, `cmd/snowflake-service/main.go` as composition root; no `pkg/`, no package-level mutable globals, everything wired through constructors (ARCHITECTURE.md Recommended Project Structure + Anti-Patterns 1–2).

**Primary recommendation:** Implement the generator exactly as ARCHITECTURE.md Pattern 1 prescribes — mutex-guarded struct, spin-wait on sequence overflow, `Generate() (int64, error)` with the clock-skew seam rejecting via a typed `ErrClockMovedBackwards` sentinel — and gate everything on `go test -race` with an N-goroutines × M-IDs global-uniqueness test (CONTEXT.md success criterion 2; PITFALLS #6).

## Key Decisions

### Clock-Skew Policy (decision gate — RESOLVED in CONTEXT.md D-01..D-04)

The generator alone detects `now < lastMs` and owns the response; the seam lives inside `Generate()`'s mutex (ARCHITECTURE.md Pattern 3). Three strategies were analyzed:

| Strategy | Pros | Cons |
|----------|------|------|
| **Reject-with-503** (typed error → Phase 2 maps to 503) | No blocking path in the uniqueness-critical kernel; no ID minted during skew (any skew re-opens an already-minted (node, ms, seq) window — CONTEXT.md D-01); honest failure, alerts fire; simplest correct code | Transient unavailability during skew events; clients must handle 503/retry (contract cost fixed early — D-01 marks reversibility as costly) |
| **Bounded-wait** | Absorbs small NTP corrections without client-visible errors | Adds a blocking path to the kernel for a scenario that should *alert, not absorb* (CONTEXT.md D-03); latency unpredictability; still needs an error fallback after the bound (PITFALLS technical-debt table: "never without a bound") |
| **Monotonic-anchored** (anchor to Go monotonic clock like bwmarrin/snowflake) | Immune to NTP steps within a process run; no skew path at all | **Zero skew detection**; embedded ID timestamps silently drift from wall-clock time — this project explicitly wants skew *handled and visible* (ARCHITECTURE.md Pattern 3, verified design input) |

**Recommendation: reject-with-typed-error** — locked as CONTEXT.md D-01/D-02/D-03. Rationale: uniqueness is the core value; any backwards movement re-opens a minted window, so minting nothing during skew is the only safe behavior, and visibility beats silent absorption. On rejection the generator does NOT mutate state — `lastMs` stays the high-water mark, subsequent calls re-check (D-02). The error seam is the observability hook: structured log + `snowflake_clock_skew_events_total` counter wire in at v2 (METR-02); Phase 1 keeps the seam in the signature and unit-tests the rejection (D-04).

### Custom Epoch

- **Value: 2026-01-01T00:00:00Z = 1767225600000 ms** (CONTEXT.md D-05). One-way decision — changing it after consumers store IDs shifts the entire embedded timestamp space (cross-epoch duplicate risk).
- **Hardcoded constant in `internal/idgen` — NOT env-configurable** (D-06). Two instances with different epochs mint disjoint ID spaces that collide over time; fleet-wide invariance is enforced by code, not operational discipline. Do not add an `EPOCH` env var.
- **Logged at startup** alongside `node_id` (D-07); document "IDs valid until ~2095" in README and as a comment on the constant (D-08; PITFALLS #4 — the built-in expiry date must be written down).

### Node-ID Sourcing

- **Always and only an explicit `NODE_ID` env var, in every environment including K8s** (CONTEXT.md D-13). No `POD_NAME` ordinal-parsing code is written, ever. This AMENDS OPS-04 and Phase 3 success criteria 1–2 (D-14): Phase 3's model becomes N single-replica workloads, each with its own distinct `NODE_ID` in its pod template. ARCHITECTURE.md Pattern 7 (StatefulSet ordinal) is superseded — do not re-litigate.
- Consequence to record: scaling any one workload above 1 replica = guaranteed duplicates — the new most-dangerous misconfiguration (D-15). Phase 1 mitigations that land now: boot log `node_id=N` (PITFALLS "Looks Done But Isn't" checklist) and fail-fast range validation (D-16).

## Generator Kernel

- **Bit layout (41/10/12):** 41 bits timestamp (ms since custom epoch, ~69 years → valid until ~2095), 10 bits node ID (0–1023), 12 bits sequence (0–4095 per ms); sign bit always 0 → IDs are positive signed 64-bit, safe for JSON-string and BIGINT consumers (ARCHITECTURE.md Pattern 1; PITFALLS #4).
- **Mutex-guarded `Generator` struct** — `mu sync.Mutex` + `{nodeID, epoch, lastMs, seq}`; pure compute, no I/O, imports only stdlib `time`/`sync` (ARCHITECTURE.md Pattern 1 + Internal Boundaries). NOT atomic-CAS, NOT channel-owned goroutine: the format ceiling is 4096 IDs/ms regardless, the mutex saturates it at ~244 ns/op in the verified reference, and alternatives multiply complexity on the two hardest paths — overflow spin and skew handling (ARCHITECTURE.md Pattern 1 trade-off table + Anti-Pattern 3).
- **Spin-wait on sequence overflow:** when 4096 IDs are consumed in one ms, busy-spin (bounded ≤ ~1 ms) until the clock ticks, then `seq = 0`. No overflow error type for callers (ARCHITECTURE.md Pattern 2; verified reference behavior).
- **Startup validation (fail-fast):** `0 ≤ nodeID ≤ 1023`; `time.Now() ≥ customEpoch` else exit non-zero — a clock before the epoch shifts the sign bit and mints negative IDs; this is a *verified gap in the reference library*, which performs no such check (ARCHITECTURE.md Pattern 3; PITFALLS #2/#4; CONTEXT.md D-16).
- **`Generate()` error contract:** `(int64, error)`; the only error path is `ErrClockMovedBackwards` (sentinel, exact naming is the agent's discretion per CONTEXT.md). On skew: return 0 + error, no state mutation (D-02).
- **`Decode(id) → {timestamp, node_id, sequence}`** implemented in the core from day one — pure bit-shift, stateless, no lock (CONTEXT.md carried-forward; ARCHITECTURE.md Data Flow). The decode HTTP endpoint is a later phase.
- **Clock injection:** small clock func/interface on the Generator (the agent's discretion, CONTEXT.md) — required for the frozen-clock and skew tests.
- **JSON string encoding dogfooded from Phase 1:** the dev route emits `{"id": "7234567890123456789"}` — JS consumers lose precision above 2^53 with raw numbers (CONTEXT.md D-10; ARCHITECTURE.md Pattern 8; PITFALLS #3 — verified: the reference library's `MarshalJSON` emits quoted strings).

## Service Foundation

- **Config (`internal/config`):** `os.Getenv` + `strconv` into one validated immutable `Config` struct; no config library at this size (ARCHITECTURE.md Component Responsibilities). Surface: `NODE_ID` (required, 0–1023), `PORT` (env var with sane default — suggest 8080; D-11 + the agent's discretion). Nothing re-reads env after boot (ARCHITECTURE.md Key Data Flows).
- **Fail-fast boot ordering** (ARCHITECTURE.md Startup Flow): `config.Load()` → validate (node ID range, clock ≥ epoch, port valid) → invalid = log + exit non-zero (OPS-05; CrashLoop surfaces misconfig immediately) → build generator → build mux → serve.
- **`main.go` as thin composition root only:** parse config, wire dependencies via constructors (`NewGenerator(cfg)`, `NewServer(gen, ...)`), own lifecycle. No business logic, no package-level mutable globals (ARCHITECTURE.md Component Responsibilities + Anti-Pattern 2 — the reference library's globals are marked DEPRECATED by its own author).
- **Shutdown skeleton (D-12):** `signal.NotifyContext` (SIGINT/SIGTERM, stdlib since Go 1.16) → run `srv.ListenAndServe()` in a goroutine → on `<-ctx.Done()` call `srv.Shutdown()` (ARCHITECTURE.md Pattern 6, verified stdlib API). The *observable* graceful drain (readyz flip + in-flight completion, OPS-03) completes in Phase 2 — this phase lands the skeleton only.
- **Temporary dev route:** minimal stdlib mux, one route under `/dev/` (e.g., `GET /dev/id`), marked "replaced in Phase 2" in a code comment so it cannot calcify into the real API (D-09). Go 1.22+ ServeMux method patterns are the floor (STACK.md).
- **Logging:** `log/slog` with JSON handler for machine-parseable pod logs (STACK.md; the agent's discretion on setup details). Boot log MUST include `node_id` and the epoch (D-07; PITFALLS checklist).

## Test Strategy

- **Race-enabled concurrency test (the non-negotiable gate):** N goroutines × M IDs each, assert global uniqueness of the full set; run with `go test -race`, gated before any image build (CONTEXT.md success criterion 2; PITFALLS #6 — races need `-race` and real concurrency to surface; single-threaded tests "work" while hiding interleaved `lastMs`/`seq` corruption).
- **Frozen-clock spin-wait test:** injectable clock freezes time; request 4097 IDs in one mocked millisecond — the 4097th MUST come from the next millisecond (PITFALLS #5 + "Looks Done But Isn't" checklist; CONTEXT.md success criterion 3). This path never executes at low volume, so bugs in it ship silently without this test.
- **Clock-skew rejection test:** mocked clock steps backwards → `Generate()` returns the typed error, no ID minted, `lastMs` unmutated; subsequent calls re-check (D-01/D-02/D-04).
- **Startup validation tests:** `NODE_ID` missing / non-numeric / outside 0–1023 → exit non-zero; mocked pre-epoch clock at boot → exit non-zero (OPS-05, D-16; PITFALLS #1/#4 — "boot crash-loop after epoch misconfiguration: working as intended — loud failure").
- **Positivity test:** every generated ID > 0 (PITFALLS #4).
- **Decode round-trip:** `Decode(Generate())` returns the expected {timestamp, node_id, sequence} (CONTEXT.md success criterion 1).
- **Dev-route smoke:** response body shows a quoted JSON string ID (D-10; GEN-03 verified end-to-end from Phase 1).

## Pitfalls

Phase-relevant pitfalls only (full list in PITFALLS.md; #3 lands in Phase 2 but is dogfooded early via D-10):

- **#1 Duplicate node IDs (THE catastrophic one):** Phase 1's share is config validation (`0 ≤ nodeID ≤ 1023`, fail fast) and the boot log line `node_id=N` on every start. Shared-ID replicas mint identical IDs that surface weeks later as consumer DB unique-constraint violations (PITFALLS #1). Under D-15, >1 replica per workload is the new most-dangerous misconfiguration — record it, Phase 3 mitigates.
- **#2 Clock skew / backwards wall clock:** Phase 1 owns the seam, the policy (reject, D-01), startup `now ≥ epoch` validation, and unit-tested rejection. Real-world triggers: NTP large-offset correction, VM suspend/resume, cloud live-migration (PITFALLS #2). Metric + alert land in v2 (D-04).
- **#4 Negative IDs from epoch/sign-bit mistakes:** clock before epoch → sign bit set → negative IDs. Prevented by startup validation + deliberate epoch + positivity test + the documented "~2095" expiry (PITFALLS #4; D-05..D-08).
- **#5 Sequence exhaustion mishandling:** sequence wrapping to 0 without waiting → duplicates within the millisecond. Verified standard behavior is the bounded spin-wait; the frozen-clock 4097-ID test is the proof (PITFALLS #5).
- **#6 Go race conditions in the generator:** unsynchronized `lastMs`/`seq` → nondeterministic duplicates. `sync.Mutex` around the whole critical section (verified reference design — do NOT get clever with CAS/channels) + `go test -race` uniqueness test in the CI gate (PITFALLS #6).

**Warning sign to bake in:** any negative ID in a response is an immediate red flag; any boot crash-loop from config validation is the system working as intended (PITFALLS #4).

## Sources

- `.planning/phases/01-generator-core-service-foundation/01-CONTEXT.md` — all D-01..D-16 locked decisions, phase boundary, the agent's discretion areas
- `.planning/research/ARCHITECTURE.md` — Patterns 1–3, 6, 8; project structure; startup flow; Anti-Patterns 1–3; verified against bwmarrin/snowflake source, go.dev, pkg.go.dev on 2026-07-26
- `.planning/research/PITFALLS.md` — Pitfalls 1, 2, 4, 5, 6; "Looks Done But Isn't" checklist; technical-debt table

---
*Research for: Phase 1 — Generator Core & Service Foundation*
*Researched: 2026-07-26*
