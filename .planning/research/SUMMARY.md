# Project Research Summary

**Project:** Snowflake ID Service
**Domain:** Distributed unique ID generation as a service (Snowflake 41/10/12) — Go REST microservice on Kubernetes
**Researched:** 2026-07-26
**Confidence:** MEDIUM-HIGH overall — ARCHITECTURE.md verified against primary sources (HIGH); STACK/FEATURES/PITFALLS synthesized inline after repeated parallel-subagent failures, fully consistent with the verified architecture.

## Executive Summary

This is a small, single-binary, stateless-per-process Go service with one stateful kernel: a mutex-guarded Snowflake generator (41-bit timestamp from custom epoch, 10-bit node ID, 12-bit sequence). Uniqueness across the fleet comes entirely from distinct static node IDs — never from runtime coordination, which is the project's founding constraint. The reference implementation (bwmarrin/snowflake) was read at source level and serves as design reference, but the generator is implemented in-house (~100 lines): the project is explicitly a Go learning vehicle, needs a custom epoch with startup validation (a verified gap in the library), and needs a clock-skew policy seam the library doesn't offer.

The recommended stack is deliberately minimal: Go stdlib only (`net/http` 1.22 mux, `log/slog`, `os/signal`) plus exactly one external dependency (`prometheus/client_golang`). Deployment is a StatefulSet with the downward API injecting pod name → ordinal as node ID — the only K8s model satisfying static-config + no-coordination + self-healing. A plain Deployment with a shared `NODE_ID` env var is an architecturally invalid duplicate-ID generator and is the project's #1 pitfall.

Key risks and mitigations: duplicate node IDs (StatefulSet ordinal + boot validation + per-node metrics + e2e duplicate check), backwards clock (explicit policy seam — reject-503 vs bounded-wait decided in planning — + skew metric + startup epoch validation), and JSON precision loss above 2^53 (IDs as strings at the API boundary, from day one).

## Key Findings

### Recommended Stack

Go (1.22+ floor, latest stable) with stdlib-only HTTP/routing/logging and a single external dependency for metrics. In-house generator core; the reference library informs design but is not imported. Distroless multi-stage Docker image; plain K8s YAML (no Helm until a second environment exists).

**Core technologies:**
- Go 1.22+ `net/http` ServeMux: routing — method patterns + wildcards cover all ~5 routes, zero router dependency
- `log/slog`: structured logging — stdlib JSON logs for pod observability
- `prometheus/client_golang`: metrics — the ONLY external dep; per-node counters prove uniqueness in production
- In-house `internal/idgen`: generator — custom epoch, skew seam, startup validation; ~100 lines, learning value per project goals

### Expected Features

**Must have (table stakes):**
- POST /v1/ids single + batch (capped `count`), IDs as JSON strings — the core value made callable
- /healthz + /readyz + graceful shutdown — K8s-safe lifecycle, near-zero-downtime goal
- Fail-fast config validation (node ID range, clock ≥ epoch) — no silent garbage
- StatefulSet/Service/PDB manifests — resilience is part of the MVP per PROJECT.md

**Should have (competitive):**
- /metrics with per-node counters + skew/overflow counters — proves uniqueness in prod
- Config introspection endpoint — ops verifies resolved node ID without shell access
- OpenAPI spec — Foundation consumers generate clients

**Defer (v2+):**
- Decode endpoint — declared nice-to-have, zero risk, pure function already tested in phase 1
- Helm packaging, `pkg/idgen` promotion — only when real demand appears

### Architecture Approach

`cmd/` composition root + `internal/{idgen,httpapi,config,observability}` + `deployments/` — no `pkg/`, no framework. The generator is a pure, deterministic kernel (imports only `time`, `sync`); handlers depend on a narrow `Generator` interface; config is one validated struct loaded once at boot.

**Major components:**
1. `internal/idgen` — mutex-guarded generator, bit layout, skew policy seam, spin-wait overflow, Decode
2. `internal/httpapi` — stdlib mux, JSON string contract, health/ops handlers
3. `internal/config` — env loading, NODE_ID | POD_NAME-ordinal resolution, fail-fast validation
4. `internal/observability` — slog + Prometheus registry
5. `deployments/` — StatefulSet + Service + PDB (the node-ID mechanism lives here)

### Critical Pitfalls

1. **Duplicate node IDs across replicas** — StatefulSet ordinal via downward API + boot validation + per-node metrics + e2e duplicate check (NEVER a Deployment with shared NODE_ID)
2. **Backwards clock → duplicate IDs** — skew seam in the core, policy decided in planning, startup `now ≥ epoch` validation, skew counter + alert
3. **JSON 2^53 precision loss** — IDs as strings at the API boundary from day one
4. **Negative IDs from epoch/sign-bit mistakes** — deliberate epoch choice, startup validation, positivity test, documented ~2095 expiry
5. **Untested sequence-exhaustion path** — spin-wait + frozen-clock test requesting 4097 IDs in one mocked millisecond

## Implications for Roadmap

Based on research (build order from ARCHITECTURE.md, coarse granularity per config → 3 phases):

### Phase 1: Generator Core + Service Skeleton
**Rationale:** The generator's correctness gates everything (core value = uniqueness); the clock-skew policy decision blocks only this phase's API shape, not the architecture
**Delivers:** `internal/idgen` (41/10/12, mutex, spin-wait, skew seam + DECIDED policy, startup epoch validation, Decode) with race-enabled concurrency + frozen-clock tests; `internal/config` with node-ID resolution (NODE_ID | POD_NAME ordinal) and fail-fast validation; `cmd` composition root with graceful shutdown
**Addresses:** ID generation correctness; config validation
**Avoids:** Pitfalls 2, 4, 5, 6

### Phase 2: REST API — POST /v1/ids + Health
**Rationale:** First deployable slice of the declared MVP; health endpoints land here because Phase 3's manifests reference them; the string-JSON contract must ship with the FIRST endpoint (retrofitting is a breaking change)
**Delivers:** POST /v1/ids (single + capped batch, string IDs), /healthz, /readyz, request logging; Dockerfile
**Uses:** stdlib mux, slog, narrow Generator interface
**Implements:** `internal/httpapi`
**Avoids:** Pitfall 3

### Phase 3: Multi-Instance on Kubernetes + Observability
**Rationale:** The resilience goal (PROJECT.md) and the riskiest ops assumption (ordinal→node-ID) deserve the earliest end-to-end proof; per-node metrics become meaningful exactly here and complete the uniqueness-proof story
**Delivers:** StatefulSet + Service + PDB manifests (downward API ordinal), e2e duplicate-check across N pods, /metrics with per-node + skew + sequence-wait counters, config introspection endpoint
**Uses:** prometheus/client_golang (first external dep)
**Implements:** `deployments/`, `internal/observability`
**Avoids:** Pitfall 1 (end-to-end), completes Pitfalls 2/5 observability

**Post-roadmap (v2, unphased):** decode endpoint, OpenAPI spec, Helm packaging — scheduled when convenient via `/gsd-phase --add`.

### Phase Ordering Rationale

- idgen core first: it is the dependency of everything and the risk concentrates there (all uniqueness-critical pitfalls map to Phase 1)
- API before K8s: manifests reference health endpoints; a runnable binary precedes a runnable fleet
- Metrics after multi-instance: per-node labels gain meaning only when multiple nodes exist; the e2e duplicate check validates the ordinal mechanism before metrics merely observe it

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 1:** Clock-skew policy decision (reject-503 vs bounded-wait vs monotonic-anchored) — architecture supports all three; planning must pick with explicit trade-offs
- **Phase 3:** Target cluster specifics (Prometheus scrape method: annotations vs ServiceMonitor; NTP/chrony on nodes is a cluster-admin prerequisite to verify)

Phases with standard patterns (skip research-phase):
- **Phase 2:** stdlib HTTP patterns verified against go.dev primary sources — nothing left to research

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM-HIGH | Approach verified via ARCHITECTURE.md primary sources; exact latest version numbers NOT re-verified inline — confirm at phase planning |
| Features | MEDIUM-HIGH | Synthesized inline from verified architecture + stable domain practice; categorization is low-controversy |
| Architecture | HIGH | Verified against bwmarrin/snowflake source, go.dev, kubernetes.io on 2026-07-26 |
| Pitfalls | MEDIUM-HIGH | Top pitfalls (node-ID collision, skew, 2^53) verified as design concerns in ARCHITECTURE.md; incident anecdotes are established domain knowledge |

**Overall confidence:** MEDIUM-HIGH

### Gaps to Address

- Exact current Go stable version: confirm at Phase 1 planning (`go.dev/dl`); pin identical toolchain in go.mod, CI, and Docker builder
- Clock-skew policy: open decision, owned by Phase 1 planning (not a research gap — a decision gate)
- Cluster Prometheus setup (annotations vs ServiceMonitor): unknown until target cluster identified; default to scrape annotations (lowest assumption)

## Sources

### Primary (HIGH confidence)
- bwmarrin/snowflake README + source — generator design, JSON strings, overflow spin, missing epoch validation (2026-07-26)
- go.dev blog — Go 1.22 routing enhancements (2026-07-26)
- kubernetes.io — StatefulSet ordinal/identity guarantees (2026-07-26)
- pkg.go.dev — promhttp v1.24.1, os/signal.NotifyContext (2026-07-26)

### Secondary (MEDIUM confidence)
- Domain practice: Twitter Snowflake history, Meituan Leaf / Baidu UidGenerator worker-ID approaches, sonyflake layout rationale — well-established, not re-fetched inline
- golang-standards/project-layout README caveats (via ARCHITECTURE.md verification)

---
*Research completed: 2026-07-26*
*Note: STACK/FEATURES/PITFALLS synthesized inline by the orchestrator after 6 of 8 parallel research subagent spawns failed (cancelled/interrupted); ARCHITECTURE.md completed normally with primary-source verification. All four documents are mutually consistent.*
*Ready for roadmap: yes*
