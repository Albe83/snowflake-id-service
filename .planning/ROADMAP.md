# Roadmap: Snowflake ID Service

## Overview

Build the uniqueness-critical Snowflake generator kernel first (with fail-fast config and the clock-skew policy decision), then expose it as a containerized REST API with honest health lifecycle, and finally prove global uniqueness end-to-end on a multi-instance Kubernetes StatefulSet with runtime config introspection. Each phase is independently verifiable; uniqueness — the core value — is tested in isolation in Phase 1 and proven across the fleet in Phase 3.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Generator Core & Service Foundation** — Mutex-guarded 41/10/12 Snowflake generator with skew policy, fail-fast config, composition root
- [ ] **Phase 2: REST API & Container** — POST /v1/ids (single + batch, string IDs), health endpoints with graceful drain, distroless Docker image
- [ ] **Phase 3: Kubernetes Multi-Instance & Introspection** — StatefulSet with ordinal-based node IDs, e2e uniqueness proof across N pods, config introspection endpoint

## Phase Details

### Phase 1: Generator Core & Service Foundation
**Goal**: The uniqueness-critical generator kernel is correct and provable in isolation, and the service binary boots with validated configuration or fails fast
**Depends on**: Nothing (first phase)
**Requirements**: GEN-03, OPS-05
**Success Criteria** (what must be TRUE):
  1. Generator produces positive signed 64-bit IDs with the 41/10/12 layout from the custom epoch (timestamp, node ID, sequence all recoverable via Decode)
  2. Concurrent generation from many goroutines produces zero duplicates (race-enabled concurrency test passes)
  3. Requesting more than 4096 IDs within one mocked millisecond succeeds via spin-wait (no error, no duplicate, frozen-clock test)
  4. Process exits non-zero at startup when node ID is outside 0–1023 or system clock reads before the custom epoch
  5. Clock-skew policy (reject-with-503 vs bounded-wait vs monotonic-anchored) is decided with documented trade-offs and implemented at the generator seam
**Plans**: TBD

> ⚠️ **Decision gate (must resolve during Phase 1 planning):** clock-skew policy — reject-with-503 vs bounded-wait (vs monotonic-anchored). Architecture supports all three; the choice shapes `Generate()`'s error contract and the Phase 2 handler mapping. PROJECT.md explicitly defers this to planning with pro/contro. See research/ARCHITECTURE.md Pattern 3 for the verified design input.

### Phase 2: REST API & Container
**Goal**: Consumers can generate IDs over REST with a stable string-JSON contract, and the service runs as a container with a Kubernetes-safe lifecycle
**Depends on**: Phase 1
**Requirements**: GEN-01, GEN-02, GEN-04, OPS-01, OPS-02, OPS-03, OPS-06
**Success Criteria** (what must be TRUE):
  1. Consumer can request a single ID via `POST /v1/ids` and receives it as a JSON string (no 2^53 precision risk)
  2. Consumer can request a batch of up to 1000 IDs in one call; a count < 1 or above the cap receives `400` with an explicit error body
  3. Orchestrator can verify liveness via `GET /healthz` and readiness via `GET /readyz`
  4. On SIGTERM, `/readyz` starts failing immediately and in-flight requests complete before the process exits (graceful drain)
  5. Operator can build and run the service as a container from a multi-stage Dockerfile (static binary, distroless/scratch base)
**Plans**: TBD

### Phase 3: Kubernetes Multi-Instance & Introspection
**Goal**: The service runs as a resilient multi-instance fleet on Kubernetes where every pod has a distinct, stable node ID, global uniqueness is proven end-to-end, and operators can inspect resolved runtime config
**Depends on**: Phase 2
**Requirements**: GEN-05, OPS-04, CONF-01
**Success Criteria** (what must be TRUE):
  1. Operator can deploy N replicas via StatefulSet manifests; each pod resolves a distinct node ID from its ordinal via the downward API (explicit NODE_ID override still works for local dev)
  2. A rescheduled/replaced pod keeps its ordinal and therefore its node ID (stable identity across self-healing)
  3. Hammering N pods concurrently and collecting the generated IDs shows zero duplicates (e2e uniqueness proof across the fleet)
  4. Operator can read the resolved runtime configuration (node_id, epoch) from each pod via the config introspection endpoint
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Generator Core & Service Foundation | 0/? | Not started | - |
| 2. REST API & Container | 0/? | Not started | - |
| 3. Kubernetes Multi-Instance & Introspection | 0/? | Not started | - |
