# Pitfalls Research

**Domain:** Snowflake-style distributed unique ID generation service (Go, REST, multi-instance Kubernetes)
**Researched:** 2026-07-26
**Confidence:** MEDIUM-HIGH — synthesized inline (parallel research subagents failed) from the primary-source-verified ARCHITECTURE.md plus established domain incident patterns. The two catastrophic pitfalls (#1 duplicate node IDs, #2 clock skew) are verified design concerns in ARCHITECTURE.md.

## Critical Pitfalls

### Pitfall 1: Duplicate Node IDs Across Replicas (THE catastrophic one)

**What goes wrong:**
Two running pods hold the same node ID. Both draw from the same (node, ms, sequence) space and emit **identical IDs**. Uniqueness — the service's entire core value — silently breaks. Worse: duplicates are sparse and interleaved, so they surface weeks later as mystery unique-constraint violations in a Foundation gestionale's database.

**Why it happens:**
The natural K8s reflex is a `Deployment` with `replicas: 3` and one `NODE_ID` env var in the pod template — every replica inherits the SAME value. "It's stateless anyway" thinking. Verified in ARCHITECTURE.md as the single most dangerous misconfiguration for this architecture.

**How to avoid:**
- StatefulSet + downward API: `POD_NAME` from `metadata.name`, app parses the ordinal (`snowflake-7` → 7). Ordinals are unique 0..N-1 across the set and stable across reschedules (verified K8s behavior).
- Config validation at boot: `0 ≤ nodeID ≤ 1023`, fail fast otherwise.
- Boot log line: `resolved node_id=N` on every start.
- Per-node metrics (`ids_generated_total{node_id}`) so collisions become *visible*.
- Human-managed registry (markdown table) of assigned node IDs as bookkeeping backstop.

**Warning signs:**
Consumer DB unique-constraint errors clustered in time; two pods reporting the same `node_id` label value in Prometheus.

**Phase to address:**
Phase 1 (config validation), Phase 3 (StatefulSet manifests + e2e duplicate check across pods), Phase 4 (per-node metrics).

---

### Pitfall 2: Clock Skew / Backwards Wall Clock

**What goes wrong:**
The system clock steps backwards (NTP correction, VM snapshot restore, container node time issues). The generator now sees `now < lastMs`. Naive behavior — reusing the old timestamp — re-opens the (node, ms, seq) window and can emit duplicates of IDs already handed out.

**Why it happens:**
Wall clocks are not monotonic; developers test on machines where NTP never visibly steps. Real-world triggers: NTP large-offset correction after boot, VM suspend/resume, cloud host live-migration.

**How to avoid:**
- Generator detects `now < lastMs` and owns the response via an explicit policy seam: `Generate() (int64, error)`. Strategy (reject-with-503 vs bounded wait vs monotonic anchor) is a **planning decision with documented trade-offs** — the seam is architecture-fixed so the choice changes no boundaries.
- Startup validation: `time.Now() ≥ customEpoch`, else exit(1) (verified gap in the reference library — it performs no such check).
- `snowflake_clock_skew_events_total` counter + structured log on EVERY detection, regardless of policy.
- Cluster nodes running NTP/chrony is a cluster-admin prerequisite — document it.

**Warning signs:**
Skew counter > 0; 503 spikes on the generation endpoint; consumer-visible generation latency jumps (if wait policy).

**Phase to address:**
Phase 1 (skew seam + startup validation + policy decision), Phase 4 (skew metric + alert).

---

### Pitfall 3: JSON Precision Loss for 64-bit IDs

**What goes wrong:**
Raw `int64` marshaled as a JSON number crosses 2^53; JavaScript/web consumers parse it as an IEEE-754 double and silently get a DIFFERENT number. Downstream duplicate-detection and lookups break — caused here, observed there.

**Why it happens:**
Go's `encoding/json` handles int64 natively and correctly, so nothing looks wrong in Go-to-Go testing. The failure only appears in JS/web/tooling consumers.

**How to avoid:**
String-encode IDs at the API boundary: `{"ids": ["7234567890123456789"]}`. Accept strings on the decode endpoint. Document the contract in the OpenAPI spec. Verified mitigation: the reference library's own `MarshalJSON` emits quoted strings.

**Warning signs:**
Consumer reports "ID not found" for IDs the service logged as generated; IDs differing in trailing digits between service logs and consumer records.

**Phase to address:**
Phase 2 (the first endpoint ships the string contract from day one — retrofitting is a breaking change).

---

### Pitfall 4: Negative IDs from Epoch / Sign-Bit Mistakes

**What goes wrong:**
(a) Clock reads before the custom epoch → negative elapsed time → sign bit set → negative IDs handed to consumers. (b) Custom epoch set too close to "now" wastes the 41-bit range (69 years from epoch); set in the future, every boot fails validation. (c) 69 years after the epoch, timestamps overflow into the sign bit — the service's built-in expiry date nobody wrote down.

**Why it happens:**
Epoch is a one-line constant chosen carelessly; sign-bit overflow is a 2070s-style problem nobody models.

**How to avoid:**
- Startup validation `now ≥ epoch` → exit(1) (prevents a).
- Choose epoch deliberately (e.g. 2026-01-01T00:00:00Z), document "IDs valid until ~2095" in README and as a code constant comment (addresses b, c).
- Integration test: generated ID is always > 0.

**Warning signs:**
Negative IDs in responses (immediate red flag); boot crash-loop after epoch misconfiguration (working as intended — loud failure).

**Phase to address:**
Phase 1 (epoch constant + validation + positivity test).

---

### Pitfall 5: Sequence Exhaustion Mishandling

**What goes wrong:**
More than 4096 generation requests land in one millisecond on one node. Mishandled options: sequence wraps to 0 without waiting → duplicates within that millisecond; or an untested error path fires under the first real burst.

**Why it happens:**
At low volume this path NEVER executes in dev/test, so bugs in it ship silently. It's the classic cold-code-path problem.

**How to avoid:**
- Verified standard behavior: spin-wait (bounded, ≤ ~1 ms) until the clock ticks, then `seq = 0`. No new error type for clients.
- Unit test with an injectable clock that freezes time and requests 4097 IDs — the 4097th must come from the next millisecond.
- `snowflake_sequence_waits_total` metric so future scale surprises are visible.

**Warning signs:**
Sequence-wait counter climbing; p99 latency spiking by ~1 ms in bursts.

**Phase to address:**
Phase 1 (spin + frozen-clock test), Phase 4 (metric).

---

### Pitfall 6: Go Race Conditions in the Generator

**What goes wrong:**
Concurrent HTTP handlers hit unsynchronized `lastMs`/`seq` state → interleaved reads/writes → duplicate or out-of-order IDs, nondeterministically.

**Why it happens:**
The generator "works" in single-threaded tests; races need `-race` and real concurrency to surface.

**How to avoid:**
- `sync.Mutex` around the whole generate critical section (verified reference design — do NOT get clever with CAS/channels; the 4096/ms format ceiling makes lock-free pointless).
- Concurrency test: N goroutines × M IDs each, assert global uniqueness of the full set. Run with `go test -race` in CI, gated before image build.

**Warning signs:**
`-race` reports; flaky uniqueness test failures.

**Phase to address:**
Phase 1 (mutex + race-enabled concurrency test in CI gate).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Single `NODE_ID` env var, no ordinal parsing | Simpler config code | Blocks multi-instance entirely; invites the duplicate-node-ID deployment | Local dev / single-binary runs only — must be an explicit override, not the prod path |
| Hand-maintained OpenAPI spec | No codegen tooling | Spec can drift from implementation | Acceptable at 3 endpoints; add a CI contract test that routes match the spec when the API grows |
| Plain YAML instead of Helm | Zero templating tooling | Copy-paste per environment | Until a second environment exists |
| Spin-wait without timeout on clock skew (if "wait" policy chosen) | Simplest wait implementation | A persistent backwards clock hangs requests instead of failing them | Never without a bound — if wait is chosen in planning, it MUST be bounded with a fallback error |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| Kubernetes Deployment | Shared `NODE_ID` env for all replicas (Pitfall 1) | StatefulSet + downward API ordinal |
| Kubernetes shutdown | Readiness keeps passing during drain → kube-proxy routes requests to a dying pod | Flip readiness to failing at shutdown START; `terminationGracePeriodSeconds` > shutdown timeout; PDB `maxUnavailable: 1` |
| Prometheus | Unlabeled `ids_generated_total` counter | `node_id` label (cardinality ≤1024, safe) — unlabeled counters hide the collision signal |
| Consumer databases | Storing the ID as 32-bit INT or as string without length thought | Document: consumers use BIGINT (signed 64-bit); IDs are positive by design |
| Container clock | Assuming containers have independent clocks | Pod clock = node kernel clock; NTP/chrony on cluster NODES is the prerequisite — document it |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Batch `count` uncapped | One request holds the generator mutex for seconds; p99 latency for everyone | Cap `count` (e.g. ≤1000), validate → 400 | First bulk-insert consumer |
| Per-request logging at info level with full ID lists | Log volume explodes on batch calls | Log count + latency, not full ID arrays, at info; IDs at debug | First high-frequency consumer |
| Mutex contention (theoretical) | Queueing on `Generate()` | Nothing — verified ~244 ns/op; format ceiling binds first | ~4M IDs/s/node — unreachable for this project |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Exposing the service outside the cluster without thought | Anyone mints IDs → ID-space pollution, trivial DoS via batch | ClusterIP only; document that exposure requires a gateway decision (auth is out of scope BY DESIGN, not by oversight) |
| Config introspection leaking beyond intent | Trivial info disclosure (node ID, epoch) | Low sensitivity by design — internal service; note it in the endpoint docs |
| Distroless image with shell-less debugging surprise | Not a vulnerability, but ops teams waste time | Document `kubectl debug` ephemeral-container workflow in the README |

## "Looks Done But Isn't" Checklist

- [ ] **Generation endpoint:** Often missing string-ID JSON encoding — verify response body shows quoted IDs, and a >2^53 ID survives a JS `JSON.parse` round-trip
- [ ] **Generator:** Often missing frozen-clock sequence-exhaustion test — verify a test requests 4097 IDs in one mocked millisecond
- [ ] **Uniqueness:** Often missing multi-pod e2e check — verify the K8s phase hammers N pods and asserts set-wide uniqueness, not just per-pod
- [ ] **Shutdown:** Often missing readiness flip on drain — verify a request DURING rollout still completes and readiness fails first
- [ ] **Config:** Often missing boot log of resolved node_id — verify every pod log line one `node_id=N` at startup

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Duplicate node IDs deployed | HIGH | Scale to 1 immediately; reassign ordinals; consumer-side dedup audit on the affected time window (decode endpoint helps: filter by node_id + timestamp) |
| Sustained backwards clock | MEDIUM | Fix node NTP/chrony; restart pod (startup validation re-checks); skew metric confirms return to zero events |
| Sequence-wait storms | LOW | Symptom of real scale — revisit bit layout (fewer node bits, more sequence bits) as a new service version |
| Node ID space exhaustion (>1024 nodes) | HIGH (design change) | Layout change = new service version; not a runtime fix — document the 1024-node ceiling in the README |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1. Duplicate node IDs | Phase 1 (validation), Phase 3 (StatefulSet + e2e), Phase 4 (per-node metrics) | e2e: N pods → globally unique ID set; Prometheus: one timeseries per node_id |
| 2. Clock skew | Phase 1 (seam + policy decision + startup validation), Phase 4 (metric + alert) | Frozen/backwards-clock unit tests; skew counter exists and increments in test |
| 3. JSON precision | Phase 2 (string contract from day one) | Contract test: ID > 2^53 round-trips as string |
| 4. Negative IDs / epoch | Phase 1 (epoch constant + validation + positivity test) | Boot test with mocked pre-epoch clock → exit(1); all IDs > 0 |
| 5. Sequence exhaustion | Phase 1 (spin + frozen-clock test), Phase 4 (metric) | Frozen-clock test passes; wait counter exists |
| 6. Generator races | Phase 1 (mutex + race test in CI) | `go test -race` green on concurrency uniqueness test |

## Sources

- ARCHITECTURE.md (this repo) — primary-source-verified: duplicate-node-ID deployment analysis, skew seam design, string JSON encoding, spin-wait behavior, StatefulSet identity model (2026-07-26)
- bwmarrin/snowflake source — mutex model, overflow spin, JSON string marshaling, missing epoch validation (verified 2026-07-26)
- Domain incident patterns (NTP corrections, VM snapshot clock jumps, JS 2^53 precision) — established operational knowledge, MEDIUM confidence
- kubernetes.io StatefulSet docs — ordinal stability guarantees (verified 2026-07-26)

---
*Pitfalls research for: Snowflake-style distributed ID generation service*
*Researched: 2026-07-26 (synthesized inline, consistent with primary-source-verified ARCHITECTURE.md)*
