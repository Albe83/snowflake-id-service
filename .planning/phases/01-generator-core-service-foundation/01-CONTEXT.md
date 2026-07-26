# Phase 1: Generator Core & Service Foundation - Context

**Gathered:** 2026-07-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Deliver the uniqueness-critical Snowflake generator kernel — mutex-guarded 41/10/12 layout with custom epoch — provably correct in isolation (unit, race-detector concurrency, and frozen-clock tests), plus the service foundation: fail-fast static config (`NODE_ID`, `PORT`), a thin composition root with signal-driven shutdown skeleton, and ONE temporary dev route for manual smoke-testing. The real REST API (`POST /v1/ids`, health endpoints, batch) is Phase 2. Kubernetes deployment is Phase 3. Metrics are v2.

Requirements covered: GEN-03 (64-bit positive IDs, JSON string serialization), OPS-05 (exit≠0 on invalid config at boot).

</domain>

<decisions>
## Implementation Decisions

### Clock-Skew Policy (roadmap decision gate — RESOLVED)
- **D-01:** On detected backwards wall clock (`now < lastMs`), `Generate()` **rejects with a typed error** (`ErrClockMovedBackwards` or equivalent). Zero tolerance — any backwards movement rejects, since any skew re-opens an already-minted (node, ms, seq) window. — **Reversibility:** costly — it fixes `Generate()`'s error contract, which Phase 2 maps to `503` and clients may build retry logic around; changing the policy after Phase 2 ships means changing a published API behavior.
- **D-02:** On rejection, the generator does **not** mutate state: `lastMs` remains the high-water mark; subsequent calls re-check the clock. No ID is minted during skew.
- **D-03:** Rejected alternatives: bounded-wait-then-reject (adds a blocking path to the uniqueness-critical kernel for a scenario that should alert, not absorb), monotonic-anchored (zero skew detection + ID timestamps drift from wall time — this project explicitly wants skew *handled and visible*), unbounded wait (can hang requests forever — research: never acceptable).
- **D-04:** Skew events must be *visible*: the error seam is the hook. Structured log + counter (`snowflake_clock_skew_events_total`) are wired at the seam when the metrics phase lands (v2, METR-02); Phase 1 keeps the seam in the signature and unit-tests the rejection.

### Custom Epoch
- **D-05:** Epoch = **2026-01-01T00:00:00Z** (1767225600000 ms). — **Reversibility:** one-way — once consumers store minted IDs, changing the epoch shifts the entire embedded timestamp space; old and new IDs cannot coexist safely (cross-epoch duplicate risk). Trivial to change only before first production use.
- **D-06:** The epoch is a **hardcoded constant in `internal/idgen` — NOT configurable via env**. User reversed an initial "env-overridable default" preference after the footgun analysis: two instances with different epochs mint disjoint ID spaces that collide over time. Fleet-wide invariance is enforced by code, not operational discipline. Do not add an `EPOCH` env var.
- **D-07:** The epoch value is **logged at startup** (boot log, together with `node_id`) so every pod log line proves which epoch it runs.
- **D-08:** Document "IDs valid until ~2095" in README and as a comment on the epoch constant (PITFALLS #4: the service's built-in expiry date must be written down).

### Phase 1 Binary Scope
- **D-09:** After a successful boot the binary serves a **temporary dev route** — minimal mux, one route under a `/dev/` prefix (e.g., `GET /dev/id`) — for local curl smoke tests. Marked temporary in code (comment "replaced in Phase 2") so it cannot calcify into the real API. Rationale: hands-on Go learning project — a running, curl-able binary matters.
- **D-10:** Dev route responds with **JSON string IDs**: `{"id": "7234567890123456789"}`. This dogfoods the final contract and verifies GEN-03 end-to-end from Phase 1 (no 2^53 precision risk for JS consumers). — **Reversibility:** one-way — string-encoded IDs are the published contract; retrofitting after consumers exist is a breaking change (PITFALLS #3).
- **D-11:** `PORT` joins the Phase 1 config surface (env var with a sane default), since a listener exists.
- **D-12:** Shutdown skeleton in Phase 1: `signal.NotifyContext` (SIGINT/SIGTERM) → `srv.Shutdown()`. The *observable* graceful drain (readyz flip + in-flight completion, OPS-03) completes in Phase 2 with the HTTP layer — per STATE.md, this phase lands the skeleton only.

### Node-ID Resolution
- **D-13:** Node ID comes **always and only from an explicit `NODE_ID` env var** — in every environment, including Kubernetes. No `POD_NAME` ordinal parsing code is written, ever. User's words: *"il node id viene passato come variabile d'ambiente"*, confirmed as the "always explicit" model. — **Reversibility:** costly — it supersedes the StatefulSet-ordinal design; undoing it later means writing the ordinal-parsing code, rewriting Phase 3 manifests, and re-deciding the deployment model.
- **D-14:** **This decision AMENDS OPS-04 (REQUIREMENTS.md) and Phase 3 success criteria 1–2 (ROADMAP.md)**, which describe the downward-API ordinal model. Phase 3's deployment model becomes: **N single-replica workloads (e.g., N Deployments), each with its own distinct `NODE_ID` in its pod template**. Self-healing preserves node ID because it lives in the template; scaling = adding a new manifest with a fresh node ID. REQUIREMENTS.md/ROADMAP.md wording updates were flagged to the user during discussion and should accompany Phase 3 planning.
- **D-15:** Consequence for Phase 3 (record, don't re-litigate): scaling any one workload above 1 replica = guaranteed duplicates. This replaces "shared NODE_ID Deployment" as the most dangerous misconfiguration. Mitigations stay as researched: boot log `node_id=N` on every start (PITFALLS checklist), per-node metrics for collision visibility (v2), and a **human-managed node-ID registry** (markdown table of assigned IDs) which stops being a backstop and becomes *the* assignment mechanism — Phase 3 must produce and maintain it.
- **D-16:** Phase 1 config validation (OPS-05, already locked): `NODE_ID` missing, non-numeric, or outside 0–1023 → exit non-zero. Also locked from research: system clock before the custom epoch at boot → exit non-zero (guards the sign bit / negative IDs, PITFALLS #4 — a verified gap in the reference library).

### Carried Forward from Research (locked — do not re-litigate)
- Mutex-guarded `Generator` struct — not atomic-CAS, not channel-owned goroutine (ARCHITECTURE.md Pattern 1 + Anti-Pattern 3; format ceiling is 4096 IDs/ms regardless).
- Spin-wait on sequence exhaustion (bounded, ≤~1 ms), not an error — Pattern 2; proven by the frozen-clock test (success criterion 3).
- Layout: `cmd/` + `internal/` only (`internal/idgen`, `internal/config`, `cmd/snowflake-service/main.go` as thin composition root); no `pkg/`, no package-level mutable globals, everything wired through constructors (Anti-Patterns 1–2).
- Go 1.22+ stdlib-first; **zero external runtime dependencies in Phase 1** (prometheus/client_golang arrives with the metrics work; stdlib mux with the real HTTP layer in Phase 2).
- `Decode(id) → {timestamp, node_id, sequence}` implemented in the core from day one (success criterion 1); the decode HTTP endpoint is a later phase.
- `go test -race` with an N-goroutines × M-IDs global-uniqueness test is the non-negotiable gate (success criterion 2).

### the agent's Discretion
- `PORT` default value (suggest 8080) and env var naming details.
- Exact dev-route path/method within the `/dev/` prefix.
- Clock-injection mechanism for the frozen-clock and skew unit tests (small clock func/interface on the Generator — needed for success criteria 3 and D-04 testing).
- Internal error naming/shape (`ErrClockMovedBackwards` sentinel), test file organization, slog setup details (JSON handler for machine-parseable logs per STACK.md).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project artifacts
- `.planning/ROADMAP.md` — Phase 1 goal + 5 success criteria; the clock-skew decision gate (resolved here as D-01..D-04). Phase 3 criteria 1–2 are superseded per D-14.
- `.planning/REQUIREMENTS.md` — GEN-03, OPS-05 (this phase). OPS-04 superseded per D-14.
- `.planning/PROJECT.md` — founding constraints (no coordination, static config, K8s, signed 64-bit) and Key Decisions table.
- `.planning/STATE.md` — Phase 1/2 boundary note for the shutdown skeleton (D-12).

### Research (verified 2026-07-26 against primary sources)
- `.planning/research/ARCHITECTURE.md` — Patterns 1–3 (mutex generator + verified code shape, spin-wait, skew seam), recommended project structure, startup fail-fast flow, Anti-Patterns 1–3. Note: Pattern 7 (StatefulSet ordinal) is superseded by D-13/D-14.
- `.planning/research/PITFALLS.md` — Pitfalls 2 (clock skew), 4 (epoch/negative IDs), 5 (sequence exhaustion), 6 (races) with their Phase 1 verification requirements; "Looks Done But Isn't" checklist (frozen-clock 4097-ID test, boot log of node_id).
- `.planning/research/STACK.md` — Go 1.22 floor, stdlib-only Phase 1, `go test -race` + golangci-lint gate, no-pkg rule.

</canonical_refs>

<code_context>
## Existing Code Insights

**Greenfield repository** — contains only `AGENTS.md` and `LICENSE`. There is no existing Go code, no go.mod, no patterns to conform to yet.

### Reusable Assets
- None. This phase creates the first code and therefore *sets* the patterns (package layout, composition root, test conventions) that later phases follow — keep them exactly as the research prescribes.

### Established Patterns
- None in code. Prescriptive patterns come from `research/ARCHITECTURE.md` (see Canonical References).

### Integration Points
- None yet. The only cross-artifact contract created here: env var names (`NODE_ID`, `PORT`) that Phase 2/3 manifests and docs will reference.

</code_context>

<specifics>
## Specific Ideas

- User values the hands-on learning dimension of this project — the temporary dev route (D-09) exists so the Phase 1 binary is curl-able, not just test-green.
- User is comfortable reversing an initial preference when shown a concrete failure mode: epoch went from "env-overridable default" to "hardcoded constant, but logged at startup" (*"non rendiamolo configurabile, usiamo come costante nel codice, ma logghiamolo all'avvio"*). When proposing config knobs, lead with the failure mode, not the flexibility.
- User chose the explicit-`NODE_ID` deployment model with eyes open about the OPS-04 amendment — simplicity of mental model ("the node ID is an env var, period") over K8s-native elegance. Phase 3 planning must respect this, not re-propose the ordinal model.

</specifics>

<deferred>
## Deferred Ideas

- Wiring `snowflake_clock_skew_events_total` + structured skew-event log at the rejection seam — v2, with the rest of metrics (METR-02). The seam (D-04) is Phase 1; the instrumentation is not.
- Observable graceful drain (readyz flip + in-flight completion, OPS-03) — Phase 2, per STATE.md boundary note.
- K8s manifests under the amended deployment model + the human-managed node-ID registry — Phase 3 (D-14/D-15).

</deferred>

---

*Phase: 1-Generator Core & Service Foundation*
*Context gathered: 2026-07-26*
